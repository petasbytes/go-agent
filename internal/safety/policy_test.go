package safety_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/petasbytes/go-agent/internal/safety"
)

func TestValidateWritePath_DenyList(t *testing.T) {
	root := t.TempDir()
	_ = os.Mkdir(filepath.Join(root, ".git"), 0o755)
	_ = os.MkdirAll(filepath.Join(root, ".agent", "sub"), 0o755)

	cases := []struct {
		name string
		rel  string
		code string
	}{
		{"git head", ".git/HEAD", "ERR_DENIED_WRITE"},
		{"git config", ".git/config", "ERR_DENIED_WRITE"},
		{"agent conversation", ".agent/conversation.json", "ERR_DENIED_WRITE"},
		{"agent subdir", ".agent/sub/state.json", "ERR_DENIED_WRITE"},
		{"go.mod at root", "go.mod", "ERR_DENIED_WRITE"},
		{"go.sum deep", "sub/dir/go.sum", "ERR_DENIED_WRITE"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := safety.ValidateWritePath(root, tc.rel); err == nil {
				t.Fatalf("expected deny for %q", tc.rel)
			} else if !strings.Contains(err.Error(), tc.code) {
				t.Fatalf("expected error code %s, got: %v", tc.code, err)
			}
		})
	}
}

func TestValidateWritePath_AbsoluteRejected(t *testing.T) {
	root := t.TempDir()
	abs, err := filepath.Abs(".")
	if err != nil {
		t.Skipf("cannot compute abs: %v", err)
	}
	if _, err := safety.ValidateWritePath(root, abs); err == nil {
		t.Fatal("expected reject for absolute path")
	} else if !strings.Contains(err.Error(),
		"ERR_PATH_OUTSIDE_SANDBOX") {
		t.Fatalf("expected ERR_PATH_OUTSIDE_SANDBOX, got: %v", err)
	}
}

func TestValidateWritePath_SymlinkEscapeOnNewFile(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()

	if runtime.GOOS == "windows" {
		t.Skip("symlink test skipped on Windows")
	}
	link := filepath.Join(root, "out")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlink not allowed on this FS: %v", err)
	}

	// Leaf does not exist; parent is a symlink pointing outside
	if _, err := safety.ValidateWritePath(root, "out/newfile.txt"); err == nil {
		t.Fatal("expected reject for symlink escape via ancestor")
	} else if !strings.Contains(err.Error(), "ERR_PATH_OUTSIDE_SANDBOX") {
		t.Fatalf("expected ERR_PATH_OUTSIDE_SANDBOX, got %v", err)
	}
}

func TestValidateWritePath_AllowNormal(t *testing.T) {
	root := t.TempDir()
    // Normalize root to avoid /var vs /private/var mismatches on macOS
    if r, err := filepath.EvalSymlinks(root); err == nil {
        root = r
    }
    _ = os.MkdirAll(filepath.Join(root, "sub", "dir"), 0o755)

    p, err := safety.ValidateWritePath(root, "sub/dir/new.txt")
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if !strings.HasPrefix(p, root+string(filepath.Separator)) {
        t.Fatalf("resolved path %q not under root %q", p, root)
    }
}
