// Package safety provides helpers for sandboxed file access.
package safety

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ToolError is a machine-readable error body for surfacing back to the agent as JSON.
type ToolError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Error returns a compact, single-line JSON string to keep tool_result payloads small.
func (e ToolError) Error() string {
	b, _ := json.Marshal(e)
	return string(b)
}

// InitSandboxRoot resolves absolute sandbox roots for read and write operations.
func InitSandboxRoot(readRoot, writeRoot string) (absRead string, absWrite string, err error) {
	// Default readRoot to CWD when empty
	if readRoot == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", "", fmt.Errorf("getwd: %w", err)
		}
		readRoot = cwd
	}

	// Default writeRoot to readRoot when empty
	if writeRoot == "" {
		writeRoot = readRoot
	}

	// Make absolute
	readRoot, err = filepath.Abs(readRoot)
	if err != nil {
		return "", "", fmt.Errorf("abs(readRoot): %w", err)
	}
	writeRoot, err = filepath.Abs(writeRoot)
	if err != nil {
		return "", "", fmt.Errorf("abs(writeRoot): %w", err)
	}

	// Resolve symlinks where possible so future boundary checks are reliable.
	// If EvalSymLinks fails (e.g., non-existent), fall back to the absolute path as-is.
	if r, err := filepath.EvalSymlinks(readRoot); err == nil {
		readRoot = r
	}
	if w, err := filepath.EvalSymlinks(writeRoot); err == nil {
		writeRoot = w
	}

	return readRoot, writeRoot, nil
}
