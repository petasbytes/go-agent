package tools_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/petasbytes/go-agent/tools"
)

func TestReadFile_Happy(t *testing.T) {
	dir := filepath.Join(sharedDir, rel(t))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("prepare: %v", err)
	}
	p := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(p, []byte("hi"), 0o644); err != nil {
		t.Fatalf("prepare: %v", err)
	}

	in := tools.ReadFileInput{Path: rel(t, "a.txt")}
	b, _ := json.Marshal(in)
	out, err := tools.ReadFileDefinition.Function(b)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if out != "hi" {
		t.Fatalf("got %q", out)
	}
}

func TestReadFile_NotFound(t *testing.T) {
	in := tools.ReadFileInput{Path: rel(t, "does-not-exist.txt")}
	b, _ := json.Marshal(in)
	_, err := tools.ReadFileDefinition.Function(b)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestReadFile_DirectoryPath_Error(t *testing.T) {
	sub := filepath.Join(sharedDir, rel(t, "sub"))
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("prepare: %v", err)
	}

	in := tools.ReadFileInput{Path: rel(t, "sub")}
	b, _ := json.Marshal(in)
	_, err := tools.ReadFileDefinition.Function(b)
	if err == nil {
		t.Fatal("expected error for directory path")
	}
	if !strings.Contains(err.Error(), "ERR_NOT_A_FILE") {
		t.Fatalf("expected ERR_NOT_A_FILE, got: %v", err)
	}
}

func TestReadFile_DenylistReadsAgent(t *testing.T) {
	if err := os.MkdirAll(filepath.Join(sharedDir, ".agent"), 0o755); err != nil {
		t.Fatalf("prepare: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sharedDir, ".agent", "conv.json"), []byte("{}"), 0o644); err != nil {
		t.Fatalf("prepare: %v", err)
	}

	in := tools.ReadFileInput{Path: ".agent/conv.json"}
	b, _ := json.Marshal(in)
	_, err := tools.ReadFileDefinition.Function(b)
	if err == nil {
		t.Fatal("expected deny for .agent/")
	}
	if !strings.Contains(err.Error(), "ERR_DENIED_READ") {
		t.Fatalf("expected ERR_DENIED_READ, got: %v", err)
	}
}
