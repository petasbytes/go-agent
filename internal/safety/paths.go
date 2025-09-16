// Package safety provides helpers for sandboxed file access.
package safety

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

// ValidateRelPath resolves relPath against absRoot and returns an absolute path
// inside the sandbox. It rejects absolute inputs, parent traversal, and symlink
// escapes, and denies reads under .git/ and .agent/. On violation, returns a ToolError.
func ValidateRelPath(absRoot, relPath string) (string, error) {
	// Reject absolute inputs early
	if filepath.IsAbs(relPath) {
		return "", ToolError{Code: "ERR_PATH_OUTSIDE_SANDBOX", Message: "absolute paths are not allowed"}
	}

	// Clean and normalise the provided relative path
	cleaned := filepath.Clean(relPath)
	// Special case: empty means "." (current dir)
	if cleaned == "" {
		cleaned = "."
	}

	// Join to make a candidate under the root
	candidate := filepath.Join(absRoot, cleaned)

	// Best-effort symlink resolution.
	// 1) Resolve the whole candidate if it exists.
	// 2) Otherwies, resolve the deepest existing ancestor (the parent dir)
	//    and rejoin the final segment. This reveals escapes via a symlinked parent.
	if resolved, err := filepath.EvalSymlinks(candidate); err == nil {
		candidate = resolved
	} else {
		// Resolve the parent if possible (useful when the leaf file doesn't exist yet)
		parent := filepath.Dir(candidate)
		if resolvedParent, err2 := filepath.EvalSymlinks(parent); err2 == nil {
			candidate = filepath.Join(resolvedParent, filepath.Base(candidate))
		}
	}

	// Boundary check using filepath.Rel (robust against partial prefix matches)
	rel, err := filepath.Rel(absRoot, candidate)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", ToolError{Code: "ERR_PATH_OUTSIDE_SANDBOX", Message: "requested path resolves outside the sandbox root"}
	}

	// Read denylist block under .git/ and .agent/
	// Check the relative form for easy prefix testing on path components
	relClean := filepath.ToSlash(rel)
	if relClean == ".git" || strings.HasPrefix(relClean, ".git/") || relClean == ".agent" || strings.HasPrefix(relClean, ".agent/") {
		return "", ToolError{Code: "ERR_DENIED_READ", Message: "reads under .git/ or .agent/ are not allowed"}
	}

	return candidate, nil
}
