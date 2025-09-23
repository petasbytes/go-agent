package tools_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/petasbytes/go-agent/tools"
)

func TestListFiles_NonRecursive_Basic(t *testing.T) {
	dir := filepath.Join(sharedDir, rel(t)) // per-test directory
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("prepare: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte(""), 0o644); err != nil {
		t.Fatalf("prepare: %v", err)
	}
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatalf("prepare: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sub", "nested.txt"), []byte(""), 0o644); err != nil {
		t.Fatalf("prepare: %v", err)
	}

	// List the per-test dir via relative path
	in := tools.ListFilesInput{Path: rel(t)}
	b, _ := json.Marshal(in)
	out, err := tools.ListFilesDefinition.Function(b)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	var got []string
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("invalid JSON output: %v; raw=%q", err, out)
	}
	set := map[string]struct{}{}
	for _, x := range got {
		set[x] = struct{}{}
	}

	if _, ok := set["a.txt"]; !ok {
		t.Fatalf("missing a.txt; got %v", got)
	}
	if _, ok := set["sub/"]; !ok {
		t.Fatalf("missing sub/; got %v", got)
	}
	if _, ok := set["sub/nested.txt"]; ok {
		t.Fatalf("unexpected nested.txt in non-recursive output; got %v", got)
	}
}

func TestListFiles_InvalidPath_Error(t *testing.T) {
	in := tools.ListFilesInput{Path: rel(t, "does", "not", "exist")}
	b, _ := json.Marshal(in)
	_, err := tools.ListFilesDefinition.Function(b)
	if err == nil {
		t.Fatal("expected error for invalid path")
	}
}

func TestListFiles_SortingAndPaging(t *testing.T) {
	dir := filepath.Join(sharedDir, rel(t))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create shuffled names
	names := []string{"c.txt", "a.txt", "b.txt", "z.txt", "m.txt"}
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(dir, n), []byte(""), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Page 1 size 2 => ["a.txt", "b.txt"]
	in := tools.ListFilesInput{Path: rel(t), Page: 1, PageSize: 2}
	raw, _ := json.Marshal(in)
	out, err := tools.ListFilesDefinition.Function(raw)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	var got1 []string
	if err := json.Unmarshal([]byte(out), &got1); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	want1 := []string{"a.txt", "b.txt"}
	if len(got1) != len(want1) || got1[0] != want1[0] || got1[1] != want1[1] {
		t.Fatalf("got=%v want=%v", got1, want1)
	}

	// Page 3 size 2 => ["z.txt"] (since sorted: a,b,c,m,z, pages are [a,b], [c,m], [z])
	in.Page = 3
	raw, _ = json.Marshal(in)
	out, err = tools.ListFilesDefinition.Function(raw)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	var got3 []string
	if err := json.Unmarshal([]byte(out), &got3); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	want3 := []string{"z.txt"}
	if len(got3) != len(want3) || got3[0] != want3[0] {
		t.Fatalf("got=%v want=%v", got3, want3)
	}

	// Out-of-range page => []
	in.Page = 4
	raw, _ = json.Marshal(in)
	out, err = tools.ListFilesDefinition.Function(raw)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if out != "[]" {
		t.Fatalf("want empty page: %q", out)
	}
}
