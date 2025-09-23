package tools_test

import (
	"encoding/json"
	"fmt"
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

func TestReadFile_DefaultLimitAndSentinel(t *testing.T) {
	dir := filepath.Join(sharedDir, rel(t))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "big.txt")

	var b strings.Builder
	for i := 1; i <= 205; i++ {
		line := fmt.Sprintf("L%d", i)
		b.WriteString(line + "\n")
	}
	if err := os.WriteFile(p, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}

	in := tools.ReadFileInput{Path: rel(t, "big.txt")}
	raw, _ := json.Marshal(in)
	out, err := tools.ReadFileDefinition.Function(raw)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	// Must end with a newline before sentinel, then sentinel
	if !strings.HasSuffix(out, "\n-- truncated; use offset/limit to fetch more --\n") {
		t.Fatalf("missing or malformed sentinel; got tail: %q", out[len(out)-80:])
	}

	// Should contain L1..L200 only (not L201)
	if !strings.Contains(out, "L1\n") || !strings.Contains(out, "L200\n") || strings.Contains(out, "L201") {
		t.Fatalf("unexpected content windowing")
	}
}

func TestReadFile_SentinelWhenLastChunkHasNoNewLine(t *testing.T) {
	dir := filepath.Join(sharedDir, rel(t))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "tiny.txt")

	// No trailing newline at end: "A\nB"
	if err := os.WriteFile(p, []byte("A\nB"), 0o644); err != nil {
		t.Fatal(err)
	}

	// limit=1 includes only "A" (with its newline), not the "B" line
	in := tools.ReadFileInput{Path: rel(t, "tiny.txt"), Limit: 1}
	raw, _ := json.Marshal(in)
	out, err := tools.ReadFileDefinition.Function(raw)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	// Should get just "A\n" plus the sentinel (i.e., newline before sentinel)
	wantSuffix := "\n-- truncated; use offset/limit to fetch more --\n"
	if !strings.HasSuffix(out, wantSuffix) {
		t.Fatalf("missing/malformed sentinel; got tail: %q", out)
	}
	if !strings.HasPrefix(out, "A\n") {
		t.Fatalf("expected first line A; got: %q", out)
	}
	if strings.Contains(out, "\nB\n") || strings.HasPrefix(out, "B\n") || strings.Contains(out, "\nB\n") {
		t.Fatalf("unexpected inclusion of B")
	}
}

func TestReadFile_OffsetLimit_NoTrucation(t *testing.T) {
	dir := filepath.Join(sharedDir, rel(t))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "a.txt")
	content := "a\nb\nc\nd\n" // 4 lines with trailing newline
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	// Choose an offset/limit that reads to the end so there is no truncation
	in := tools.ReadFileInput{Path: rel(t, "a.txt"), Offset: 2, Limit: 10}
	raw, _ := json.Marshal(in)
	out, err := tools.ReadFileDefinition.Function(raw)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	if out != "c\nd\n" { // lines 3..4, reads through end -> no sentinel
		t.Fatalf("got %q", out)
	}
}

func TestReadFile_NegativeAndBeyondEnd(t *testing.T) {
	dir := filepath.Join(sharedDir, rel(t))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(p, []byte("x\ny\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Negative offset/limit clamps to defaults and 0; for a small file this reads all content with NO truncation
	in := tools.ReadFileInput{Path: rel(t, "a.txt"), Offset: -10, Limit: -1}
	raw, _ := json.Marshal(in)
	out, err := tools.ReadFileDefinition.Function(raw)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if out != "x\ny\n" { // full content, no sentinel expected
		t.Fatalf("unexpected content for negative offset/limit: %q", out)
	}

	// Offset beyond end => empty string (no sentinel)
	in2 := tools.ReadFileInput{Path: rel(t, "a.txt"), Offset: 999, Limit: 10}
	raw2, _ := json.Marshal(in2)
	out2, err := tools.ReadFileDefinition.Function(raw2)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if out2 != "" {
		t.Fatalf("expected empty content for offset beyond end; got %q", out2)
	}
}
