package tools_test

import (
	"os"
	"path/filepath"
	"testing"
)

var sharedDir string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "tools-tests-")
	if err != nil {
		panic(err)
	}
	_ = os.Setenv("AGT_READ_ROOT", dir)
	_ = os.Setenv("AGT_WRITE_ROOT", dir)
	sharedDir = dir

	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

// Helper to create per-test relative paths
func rel(t *testing.T, elems ...string) string {
	return filepath.Join(append([]string{t.Name()}, elems...)...)
}
