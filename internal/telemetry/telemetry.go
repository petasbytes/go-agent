// Package telemetry provides helpers for attaching and retrieving telemetry
// metadata from context.
package telemetry

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Emit writes a single JSON line to .agent/events.jsonl when observation is enabled.
// It augments fields with RFC3339Nano time and the event name.
func Emit(name string, fields map[string]any) {
	if !ObserveEnabled() {
		return
	}

	// Make a shallow copy so callers' maps aren't mutated.
	m := make(map[string]any, len(fields)+2)
	maps.Copy(m, fields)
	m["time"] = time.Now().UTC().Format(time.RFC3339Nano)
	m["event"] = name

	b, err := json.Marshal(m)
	if err != nil {
		fmt.Fprintf(os.Stderr, "telemetry: marshal: %v\n", err)
		return
	}

	base := strings.TrimSpace(os.Getenv("AGT_ARTIFACTS_DIR"))
	if base == "" {
		base = ".agent"
	}
	if err := os.MkdirAll(base, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "telemetry: mkdir %s: %v\n", base, err)
		return
	}

	path := filepath.Join(base, "events.jsonl")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "telemetry: open %s: %v\n", path, err)
		return
	}
	defer f.Close()

	if _, err := f.Write(append(b, '\n')); err != nil {
		fmt.Fprintf(os.Stderr, "telemetry: write %s: %v\n", path, err)
		return
	}
}
