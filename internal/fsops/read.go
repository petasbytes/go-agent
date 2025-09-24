package fsops

import (
	"os"

	"github.com/petasbytes/go-agent/internal/safety"
)

// maxReadableFileSize bounds file reads to avoid excessive memory use.
// Keep unexported; adjust only if tool caps/design changes.
const maxReadableFileSize int64 = 20 * 1024 * 1024 // 20MB

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

	// Reject very large files to avoid excessive memory use
	if fi.Size() > maxReadableFileSize {
		return "", safety.ToolError{Code: "ERR_FILE_TOO_LARGE", Message: "file exceeds 20MB limit"}
	}

	b, err := os.ReadFile(absPath)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
