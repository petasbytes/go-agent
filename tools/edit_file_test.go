package tools_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/petasbytes/go-agent/tools"
)

func TestEditFile_CreateNew(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "new.txt")
	in := tools.EditFileInput{Path: p, OldStr: "", NewStr: "hello"}
	b, _ := json.Marshal(in)
	out, err := tools.EditFileDefinition.Function(b)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if out == "" {
		t.Fatalf("expected non-empty success message")
	}
	data, _ := os.ReadFile(p)
	if string(data) != "hello" {
		t.Fatalf("unexpected file content: %q", string(data))
	}
}

func TestEditFile_ReplaceOK(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a.txt")
	_ = os.WriteFile(p, []byte("abc abc"), 0o644)
	in := tools.EditFileInput{Path: p, OldStr: "abc", NewStr: "XYZ"}
	b, _ := json.Marshal(in)
	out, err := tools.EditFileDefinition.Function(b)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if out != "OK" {
		t.Fatalf("expected OK, got %q", out)
	}
	data, _ := os.ReadFile(p)
	if string(data) != "XYZ XYZ" {
		t.Fatalf("unexpected file content: %q", string(data))
	}
}

func TestEditFile_OldNotFound_Error(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a.txt")
	_ = os.WriteFile(p, []byte("abc"), 0o644)
	in := tools.EditFileInput{Path: p, OldStr: "nope", NewStr: "x"}
	b, _ := json.Marshal(in)
	_, err := tools.EditFileDefinition.Function(b)
	if err == nil {
		t.Fatal("expected error when old_str not found")
	}
}

func TestEditFile_InvalidParams_Error(t *testing.T) {
	// Case 1: empty path
	{
		in := tools.EditFileInput{Path: "", OldStr: "a", NewStr: "b"}
		b, _ := json.Marshal(in)
		if _, err := tools.EditFileDefinition.Function(b); err == nil {
			t.Fatal("expected error for empty path")
		}
	}
	// Case 2: OldStr == NewStr
	{
		in := tools.EditFileInput{Path: "some.txt", OldStr: "x", NewStr: "x"}
		b, _ := json.Marshal(in)
		if _, err := tools.EditFileDefinition.Function(b); err == nil {
			t.Fatal("expected error when OldStr == NewStr")
		}
	}
}
