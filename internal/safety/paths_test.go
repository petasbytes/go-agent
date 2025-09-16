package safety_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/petasbytes/go-agent/internal/safety"
)

func TestValidateRelPath_BasicRejections(t *testing.T) {
	root := t.TempDir()

	// Absolute path should be rejected (OS-independent)
	abs, err := filepath.Abs(".")
	if err != nil {
		t.Skipf("cannot compute absolute path: %v", err)
	}
	if _, err := safety.ValidateRelPath(root, abs); err == nil {
		t.Fatal("expected error for absolute path")
	}

	// Parent traversal should be rejected
	if _, err := safety.ValidateRelPath(root, "../../x"); err == nil {
		t.Fatal("expected error for parent traversal")
	}
}

func TestValidateRelPath_ReadDenylist(t *testing.T) {
	root := t.TempDir()
	_ = os.Mkdir(filepath.Join(root, ".agent"), 0o755)
	_ = os.Mkdir(filepath.Join(root, ".git"), 0o755)

	if _, err := safety.ValidateRelPath(root, ".agent/conv.json"); err == nil {
		t.Fatal("expected deny for .agent/")
	}
	if _, err := safety.ValidateRelPath(root, ".git/HEAD"); err == nil {
		t.Fatal("expected deny for .git/")
	}
}

func TestValidateRelPath_SymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()

	link := filepath.Join(root, "out")
	if runtime.GOOS == "windows" {
		t.Skip("symlink test skipped on Windows")
	}
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlink not allowed on this FS: %v", err)
	}

	target := "out/escape.txt"
	if _, err := safety.ValidateRelPath(root, target); err == nil {
		t.Fatalf("expected reject for symlink escape: %s", target)
	}
}
