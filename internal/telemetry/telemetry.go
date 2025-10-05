package telemetry

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// isObserveEnabled checks if JSONL emission is enabled.
func isObserveEnabled() bool {
	return os.Getenv("AGT_OBSERVE_JSON") == "1"
}

// Emit writes a single JSON line to .agent/events.jsonl when AGT_OBSERVE_JSON=1.
// It augments fields with RFC3339Nano time and the event name.
func Emit(name string, fields map[string]any) {
	if !isObserveEnabled() {
		return
	}

	// Make a shallow copy so callers' maps aren't mutated.
	m := make(map[string]any, len(fields)+2)
	for k, v := range fields {
		m[k] = v
	}
	m["time"] = time.Now().UTC().Format(time.RFC3339Nano)
	m["event"] = name

	b, err := json.Marshal(m)
	if err != nil {
		fmt.Fprintf(os.Stderr, "telemetry: marshal: %v\n", err)
		return
	}

	dir := ".agent"
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "telemetry: mkdir %s: %v\n", dir, err)
		return
	}

	path := filepath.Join(dir, "events.jsonl")
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
