package fsops

import (
	"os"
	"path/filepath"

	"github.com/petasbytes/go-agent/internal/safety"
)

// WriteFile writes content to a file addressed by a relative path under the sandbox write root.
// It validates the path via safety and creates parent directories as needed.
func WriteFile(relPath, content string) error {
	_, writeRoot, err := getRoots()
	if err != nil {
		return err
	}

	absPath, err := safety.ValidateWritePath(writeRoot, relPath)
	if err != nil {
		return err // propagate ToolError unchanged
	}

	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	return os.WriteFile(absPath, []byte(content), 0o644)
}
