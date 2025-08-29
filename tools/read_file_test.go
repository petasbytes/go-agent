package tools_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/petasbytes/go-agent/tools"
)

func TestReadFile_Happy(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a.txt")
	_ = os.WriteFile(p, []byte("hi"), 0o644)

	in := tools.ReadFileInput{Path: p}
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
	in := tools.ReadFileInput{Path: "does-not-exist.txt"}
	b, _ := json.Marshal(in)
	_, err := tools.ReadFileDefinition.Function(b)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestReadFile_DirectoryPath_Error(t *testing.T) {
	dir := t.TempDir() // this is a directory, not a file

	in := tools.ReadFileInput{Path: dir}
	b, _ := json.Marshal(in)
	_, err := tools.ReadFileDefinition.Function(b)
	if err == nil {
		t.Fatal("expected error for directory path")
	}
}
