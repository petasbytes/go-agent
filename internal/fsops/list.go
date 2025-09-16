package fsops

import (
	"encoding/json"
	"os"

	"github.com/petasbytes/go-agent/internal/safety"
)

// ListFiles lists non-recursive directory entries for a relative directory path under the sandbox.
// It returns a JSON-encoded []string of names, with directories suffixed by "/".
func ListFiles(relDir string) (string, error) {
	readRoot, _, err := getRoots()
	if err != nil {
		return "", err
	}

	if relDir == "" {
		relDir = "."
	}
	absDir, err := safety.ValidateRelPath(readRoot, relDir)
	if err != nil {
		return "", err
	}

	entries, err := os.ReadDir(absDir)
	if err != nil {
		return "", err
	}

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() {
			name += "/"
		}
		names = append(names, name)
	}

	b, err := json.Marshal(names)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
