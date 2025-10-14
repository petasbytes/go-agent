package telemetry_test

import (
	"encoding/json"
	"math"
	"os/exec"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/petasbytes/go-agent/internal/telemetry"
)

func TestEmit_Gating(t *testing.T) {
    // Run in a subprocess so startup-evaluated telemetry config sees AGT_OBSERVE_JSON=0.
    tmpDir := t.TempDir()
    cmd := exec.Command(os.Args[0], "-test.run=TestEmitGatingProbe")
    cmd.Env = append(os.Environ(),
        "GO_WANT_HELPER_PROCESS=1",
        "AGT_OBSERVE_JSON=0",
        // Ensure related flags are cleared so defaults don't flip it on
        "AGT_CALIBRATION_MODE=",
        "AGT_PERSIST_API_PAYLOADS=",
        // Run in tmpDir so child writes here if it were to emit
        "PWD="+tmpDir,
    )
    cmd.Dir = tmpDir
    out, err := cmd.CombinedOutput()
    if err != nil {
        t.Fatalf("subprocess error: %v\n%s", err, string(out))
    }
    if !strings.Contains(string(out), "no_file=true") {
        t.Fatalf("expected no_file=true, got output:\n%s", string(out))
    }
}

func TestEmitGatingProbe(t *testing.T) {
    if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
        return
    }
    // Child: attempt an emission with gating off
    telemetry.Emit("test_event", map[string]any{"foo": "bar"})
    if _, err := os.Stat(".agent/events.jsonl"); os.IsNotExist(err) {
        // Print a sentinel for parent to assert
        println("no_file=true")
    } else {
        println("no_file=false")
    }
}

func TestEmit_HappyPath(t *testing.T) {
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AGT_OBSERVE_JSON", "1")

	telemetry.Emit("test_event", map[string]any{"foo": "bar", "num": 42})

	data, err := os.ReadFile(".agent/events.jsonl")
	if err != nil {
		t.Fatalf("failed to read events.jsonl: %v", err)
	}

	// Should be exactly one line
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}

	var event map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &event); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	// Assert required keys
	if event["event"] != "test_event" {
		t.Errorf("expected event=test_event, got %v", event["event"])
	}
	if event["foo"] != "bar" {
		t.Errorf("expected foo=bar, got %v", event["foo"])
	}
	if event["num"] != float64(42) { // JSON numbers are float64
		t.Errorf("expected num=42, got %v", event["num"])
	}

	// Assert time field exists and is valid RFC3339Nano
	timeStr, ok := event["time"].(string)
	if !ok {
		t.Fatal("expected time field as string")
	}
	if _, err := time.Parse(time.RFC3339Nano, timeStr); err != nil {
		t.Errorf("time field not valid RFC3339Nano: %v", err)
	}
}

func TestEmit_MultipleEmissions(t *testing.T) {
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AGT_OBSERVE_JSON", "1")

	// Emit three times
	telemetry.Emit("event1", map[string]any{"id": 1})
	telemetry.Emit("event2", map[string]any{"id": 2})
	telemetry.Emit("event3", map[string]any{"id": 3})

	data, err := os.ReadFile(".agent/events.jsonl")
	if err != nil {
		t.Fatalf("failed to read events.jsonl: %v", err)
	}

	// Should be exactly three lines
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}

	// Parse and check each line
	expectedEvents := []string{"event1", "event2", "event3"}
	for i, line := range lines {
		var event map[string]any
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatalf("line %d invalid JSON: %v", i+1, err)
		}
		if event["event"] != expectedEvents[i] {
			t.Errorf("line %d: expected event=%s, got %v", i+1, expectedEvents[i], event["event"])
		}
	}
}

func TestEmit_MapIsolation(t *testing.T) {
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AGT_OBSERVE_JSON", "1")

	// Create a map and emit
	fields := map[string]any{"key": "value"}
	telemetry.Emit("test", fields)

	// Assert original map is unchanged
	if len(fields) != 1 {
		t.Errorf("expected fields to have 1 key, got %d", len(fields))
	}
	if fields["key"] != "value" {
		t.Errorf("expected key=value, got %v", fields["key"])
	}
	if _, ok := fields["time"]; ok {
		t.Error("fields should not contain 'time' key")
	}
	if _, ok := fields["event"]; ok {
		t.Error("fields should not contain 'event' key")
	}
}

func TestEmit_ErrorHandling_ReadOnlyDir(t *testing.T) {
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AGT_OBSERVE_JSON", "1")

	// Create .agent directory as read-only
	if err := os.Mkdir(".agent", 0o555); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(".agent", 0o755) // cleanup

	// Emit should not panic (just print to stderr)
	// We're not capturing stderr here, just ensuring no panic
	telemetry.Emit("test", map[string]any{"foo": "bar"})

	// If we got here without panic, test passes
}

func TestEmit_ErrorHandling_MarshalError(t *testing.T) {
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AGT_OBSERVE_JSON", "1")

	// NaN cannot be marshaled by encoding/json (will error)
	telemetry.Emit("bad", map[string]any{"x": math.NaN()})

	// Should not create file (or directory) on marshal error
	if _, err := os.Stat(".agent/events.jsonl"); !os.IsNotExist(err) {
		t.Fatalf("expected no events file on marshal error, got err=%v", err)
	}
}

func TestEmit_NilFields(t *testing.T) {
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AGT_OBSERVE_JSON", "1")

	// Pass nil map; should not panic and should write event+time only
	telemetry.Emit("nil_fields", nil)

	data, err := os.ReadFile(".agent/events.jsonl")
	if err != nil {
		t.Fatalf("failed to read events.jsonl: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}

	var event map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &event); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if event["event"] != "nil_fields" {
		t.Errorf("expected event=nil_fields, got %v", event["event"])
	}
	// Expect exactly 2 keys: event and time
	if len(event) != 2 {
		t.Fatalf("expected exactly 2 keys (event,time), got %d: %#v", len(event), event)
	}
	if _, ok := event["time"].(string); !ok {
		t.Fatal("expected time field as string")
	}
}

func TestEmit_ErrorHandling_ReadOnlyFile(t *testing.T) {
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AGT_OBSERVE_JSON", "1")

	// Prepare .agent and a read-only events.jsonl
	if err := os.Mkdir(".agent", 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(".agent/events.jsonl", os.O_CREATE|os.O_WRONLY, 0o444)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	if err := os.Chmod(".agent/events.jsonl", 0o444); err != nil {
		t.Fatal(err)
	}

	// Should not panic; open will fail and be logged to stderr
	telemetry.Emit("x", map[string]any{"a": 1})

	// File should remain unchanged (size 0)
	fi, err := os.Stat(".agent/events.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	if fi.Size() != 0 {
		t.Fatalf("expected read-only file size 0, got %d", fi.Size())
	}
}
