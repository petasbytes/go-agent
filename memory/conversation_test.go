package memory_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/petasbytes/go-agent/memory"
)

func TestConversation_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "conv.json")

	in := []memory.Message{{Role: "user", Text: "hi"}, {Role: "assistant", Text: "hello"}}
	if err := memory.SaveConversation(p, in); err != nil {
		t.Fatalf("save: %v", err)
	}

	out, err := memory.LoadConversation(p)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(out) != len(in) {
		t.Fatalf("length mismatch: got %d want %d", len(out), len(in))
	}
	for i := range in {
		if in[i] != out[i] {
			t.Fatalf("mismatch at %d: got %+v want %+v", i, out[i], in[i])
		}
	}
}

func TestConversation_LoadMissing_ReturnsNil(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "does-not-exist.json")

	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Fatalf("expected missing file in tempdir")
	}

	msgs, err := memory.LoadConversation(p)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if msgs != nil {
		t.Fatalf("expected nil slice for missing file, got %#v", msgs)
	}
}

func TestConversation_LoadInvalidJSON_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(p, []byte("{oops"), 0o664); err != nil {
		t.Fatalf("prep: %v", err)
	}
	if _, err := memory.LoadConversation(p); err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}
