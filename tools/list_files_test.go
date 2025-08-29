package tools_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/petasbytes/go-agent/tools"
)

func TestListFiles_NonRecursive_Basic(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "a.txt"), []byte(""), 0o644)
	_ = os.Mkdir(filepath.Join(dir, "sub"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "sub", "nested.txt"), []byte(""), 0o644)

	in := tools.ListFilesInput{Path: dir}
	b, _ := json.Marshal(in)
	out, err := tools.ListFilesDefinition.Function(b)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	var got []string
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("invalid JSON output: %v; raw=%q", err, out)
	}
	set := map[string]struct{}{}
	for _, x := range got {
		set[x] = struct{}{}
	}

	if _, ok := set["a.txt"]; !ok {
		t.Fatalf("missing a.txt; got %v", got)
	}
	if _, ok := set["sub/"]; !ok {
		t.Fatalf("missing sub/; got %v", got)
	}
	if _, ok := set["sub/nested.txt"]; ok {
		t.Fatalf("unexpected nested.txt in non-recursive output; got %v", got)
	}
}

func TestListFiles_InvalidPath_Error(t *testing.T) {
	in := tools.ListFilesInput{Path: filepath.Join("/does", "not", "exist")}
	b, _ := json.Marshal(in)
	_, err := tools.ListFilesDefinition.Function(b)
	if err == nil {
		t.Fatal("expected error for invalid path")
	}
}
