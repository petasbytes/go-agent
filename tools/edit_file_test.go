package tools_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/petasbytes/go-agent/tools"
)

func TestEditFile_CreateNew(t *testing.T) {
	dir := filepath.Join(sharedDir, rel(t))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("prepare: %v", err)
	}
	in := tools.EditFileInput{Path: rel(t, "new.txt"), OldStr: "", NewStr: "hello"}
	b, _ := json.Marshal(in)
	out, err := tools.EditFileDefinition.Function(b)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if out == "" {
		t.Fatalf("expected non-empty success message")
	}
	data, _ := os.ReadFile(filepath.Join(dir, "new.txt"))
	if string(data) != "hello" {
		t.Fatalf("unexpected file content: %q", string(data))
	}
}

func TestEditFile_ReplaceOK(t *testing.T) {
	dir := filepath.Join(sharedDir, rel(t))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("prepare: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("abc abc"), 0o644); err != nil {
		t.Fatalf("prepare: %v", err)
	}
	in := tools.EditFileInput{Path: rel(t, "a.txt"), OldStr: "abc", NewStr: "XYZ"}
	b, _ := json.Marshal(in)
	out, err := tools.EditFileDefinition.Function(b)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if out != "OK" {
		t.Fatalf("expected OK, got %q", out)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "a.txt"))
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

func TestEditFile_DenyWriteGit(t *testing.T) {
	// Prepare top-level .git in shared sandbox (no per-test subdir)
	if err := os.Mkdir(filepath.Join(sharedDir, ".git"), 0o755); err != nil {
		t.Fatalf("prepare: %v", err)
	}
	in := tools.EditFileInput{Path: ".git/HEAD", OldStr: "", NewStr: "ref: refs/heads/main\n"}
	b, _ := json.Marshal(in)
	_, err := tools.EditFileDefinition.Function(b)
	if err == nil {
		t.Fatal("expected deny for writes under .git/")
	}
	if !strings.Contains(err.Error(), "ERR_DENIED_WRITE") {
		t.Fatalf("expected ERR_DENIED_WRITE, got: %v", err)
	}
}

func TestEditFile_DenyWriteAgentConversation(t *testing.T) {
	if err := os.MkdirAll(filepath.Join(sharedDir, ".agent"), 0o755); err != nil {
		t.Fatalf("prepare: %v", err)
	}
	in := tools.EditFileInput{Path: ".agent/conversation.json", OldStr: "", NewStr: "{}"}
	b, _ := json.Marshal(in)
	_, err := tools.EditFileDefinition.Function(b)
	if err == nil {
		t.Fatal("expected deny for writes under .agent/")
	}
	if !strings.Contains(err.Error(), "ERR_DENIED_WRITE") {
		t.Fatalf("expected ERR_DENIED_WRITE, got: %v", err)
	}
}
