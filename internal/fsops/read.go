package fsops

import (
	"os"

	"github.com/petasbytes/go-agent/internal/safety"
)

// ReadFile reads a file addressed by a relative path under the sandbox read root.
// It validates the path via safety and returns a ToolError JSON on policy violations.
func ReadFile(relPath string) (string, error) {
	readRoot, _, err := getRoots()
	if err != nil {
		return "", err
	}

	absPath, err := safety.ValidateRelPath(readRoot, relPath)
	if err != nil {
		return "", err // propagate ToolError or standard error
	}

	fi, err := os.Stat(absPath)
	if err != nil {
		return "", err
	}
	if fi.IsDir() {
		return "", safety.ToolError{Code: "ERR_NOT_A_FILE", Message: "path is a directory"}
	}

	b, err := os.ReadFile(absPath)
	if err != nil {
		return "", err // standard error for I/O issues (not policy)
	}
	return string(b), nil
}
