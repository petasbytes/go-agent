package tools_test

import (
	"testing"

	"github.com/petasbytes/go-agent/tools"
)

func TestRegistry_ToolCount(t *testing.T) {
	defs := tools.Registry()
	wantCount := 3 // read_file, list_files, edit_file
	if len(defs) != wantCount {
		t.Fatalf("unexpected number of tools: got %d want %d", len(defs), wantCount)
	}
}

func TestRegistry_ToolNames(t *testing.T) {
	defs := tools.Registry()
	want := map[string]struct{}{
		"read_file":  {},
		"list_files": {},
		"edit_file":  {},
	}

	// Unexpected names detected
	for _, d := range defs {
		if _, ok := want[d.Name]; !ok {
			t.Fatalf("unexpected tool in registry: %q", d.Name)
		}
	}

	// Missing expected names
	got := map[string]struct{}{}
	for _, d := range defs {
		got[d.Name] = struct{}{}
	}
	for name := range want {
		if _, ok := got[name]; !ok {
			t.Errorf("missing expected tool: %q", name)
		}
	}

	// Fail now if any errors were reported above
	if t.Failed() {
		t.FailNow()
	}
}
