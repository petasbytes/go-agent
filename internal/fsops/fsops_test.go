package fsops_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/petasbytes/go-agent/internal/fsops"
	"github.com/petasbytes/go-agent/internal/safety"
)

// Shared sandbox root for all fsops tests
var sharedDir string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "fsops-tests-")
	if err != nil {
		panic(err)
	}
	// Set env once so fsops caches the same roots for all tests
	_ = os.Setenv("AGT_READ_ROOT", dir)
	_ = os.Setenv("AGT_WRITE_ROOT", dir)
	sharedDir = dir

	code := m.Run()

	// Optional cleanup; comment out to inspect artifacts after failures
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

func setupSandbox(t *testing.T) string {
	t.Helper()
	return sharedDir
}

func rel(t *testing.T, elems ...string) string {
	return filepath.Join(append([]string{t.Name()}, elems...)...)
}

func TestReadFile_HappyPath(t *testing.T) {
	dir := setupSandbox(t)
	want := "hello world"
	if err := os.MkdirAll(filepath.Join(dir, rel(t)), 0o755); err != nil {
		t.Fatalf("prepare: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, rel(t, "a.txt")), []byte(want), 0o644); err != nil {
		t.Fatalf("prepare: %v", err)
	}
	got, err := fsops.ReadFile(rel(t, "a.txt"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if got != want {
		t.Fatalf("content mismatch: got %q want %q", got, want)
	}
}

func TestReadFile_DirectoryIsNotAFile(t *testing.T) {
	dir := setupSandbox(t)
	if err := os.MkdirAll(filepath.Join(dir, rel(t, "sub")), 0o755); err != nil {
		t.Fatalf("prepare: %v", err)
	}
	_, err := fsops.ReadFile(rel(t, "sub"))
	if err == nil {
		t.Fatal("expected error for directory target")
	}
	var te safety.ToolError
	if !errors.As(err, &te) {
		t.Fatalf("expected ToolError, got %T: %v", err, err)
	}
	if te.Code != "ERR_NOT_A_FILE" {
		t.Fatalf("unexpected code: %s", te.Code)
	}
}

func TestListFiles_JSONAndSuffixes(t *testing.T) {
	dir := setupSandbox(t)
	if err := os.MkdirAll(filepath.Join(dir, rel(t)), 0o755); err != nil {
		t.Fatalf("prepare: %v", err)
	}
	// Prepare: a.txt, b.txt, sub/
	for _, name := range []string{"a.txt", "b.txt"} {
		if err := os.WriteFile(filepath.Join(dir, rel(t, name)), []byte("x"), 0o644); err != nil {
			t.Fatalf("prepare: %v", err)
		}
	}
	if err := os.MkdirAll(filepath.Join(dir, rel(t, "sub")), 0o755); err != nil {
		t.Fatalf("prepare: %v", err)
	}

	// List the per-test directory to avoid cross-test entries
	raw, err := fsops.ListFiles(rel(t))
	if err != nil {
		t.Fatalf("ListFiles(\"\"): %v", err)
	}
	var names []string
	if err := json.Unmarshal([]byte(raw), &names); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// Expect: a.txt, b.txt, sub/
	got := map[string]bool{}
	for _, n := range names {
		got[n] = true
	}
	for _, want := range []string{"a.txt", "b.txt", "sub/"} {
		if !got[want] {
			t.Fatalf("missing entry %q in %v", want, names)
		}
	}

	// ListFiles on the per-test subdir should return empty list (no contents created)
	raw2, err := fsops.ListFiles(rel(t, "sub"))
	if err != nil {
		t.Fatalf("ListFiles(sub): %v", err)
	}
	var names2 []string
	if err := json.Unmarshal([]byte(raw2), &names2); err != nil {
		t.Fatalf("unmarshal2: %v", err)
	}
	if len(names2) != 0 {
		t.Fatalf("expected empty subdir list, got %v", names2)
	}
}

func TestWriteFile_HappyPathNested(t *testing.T) {
	_ = setupSandbox(t)
	err := fsops.WriteFile(rel(t, "nested", "dir", "out.txt"), "hello")
	if err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	// Verify file and content
	b, err := os.ReadFile(filepath.Join(os.Getenv("AGT_WRITE_ROOT"), rel(t, "nested", "dir", "out.txt")))
	if err != nil {
		t.Fatalf("verify read: %v", err)
	}
	if string(b) != "hello" {
		t.Fatalf("content mismatch: got %q", string(b))
	}
}

func TestErrorPropagation_ReadDenylist(t *testing.T) {
	dir := setupSandbox(t)
	if err := os.Mkdir(filepath.Join(dir, ".agent"), 0o755); err != nil {
		t.Fatalf("prepare: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".agent/conv.json"), []byte("{}"), 0o644); err != nil {
		t.Fatalf("prepare: %v", err)
	}
	_, err := fsops.ReadFile(".agent/conv.json")
	if err == nil {
		t.Fatal("expected deny for .agent/")
	}
	var te safety.ToolError
	if !errors.As(err, &te) {
		t.Fatalf("expected ToolError, got %T: %v", err, err)
	}
	if te.Code != "ERR_DENIED_READ" {
		t.Fatalf("unexpected code: %s", te.Code)
	}
}

func TestErrorPropagation_WriteDenyList(t *testing.T) {
	_ = setupSandbox(t)

	// .git/ directory-prefiix block
	if err := fsops.WriteFile(".git/HEAD", "ref: refs/heads/main\n"); err == nil {
		t.Fatal("expected deny for writes under .git/")
	} else {
		var te safety.ToolError
		if !errors.As(err, &te) {
			t.Fatalf("expected ToolError, got %T: %v", err, err)
		}
		if te.Code != "ERR_DENIED_WRITE" {
			t.Fatalf("unexpected code: %s", te.Code)
		}
	}

	// Basename block at any depth
	if err := fsops.WriteFile("go.mod", "module x\n"); err == nil {
		t.Fatal("expected deny for writes to go.mod")
	} else {
		var te safety.ToolError
		if !errors.As(err, &te) {
			t.Fatalf("expected ToolError, got %T: %v", err, err)
		}
		if te.Code != "ERR_DENIED_WRITE" {
			t.Fatalf("unexpected code: %s", te.Code)
		}
	}
}

func TestErrorPropagation_ReadTraversal(t *testing.T) {
	_ = setupSandbox(t)
	_, err := fsops.ReadFile("../../x")
	if err == nil {
		t.Fatal("expected traversal to be denied")
	}
	var te safety.ToolError
	if !errors.As(err, &te) {
		t.Fatalf("expected ToolError, got %T: %v", err, err)
	}
	if te.Code != "ERR_PATH_OUTSIDE_SANDBOX" {
		t.Fatalf("unexpected code: %s", te.Code)
	}
}
