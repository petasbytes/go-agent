package telemetry_test

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/petasbytes/go-agent/internal/metrics"
	"github.com/petasbytes/go-agent/internal/telemetry"
)

// readLastJSONL returns the last non-empty JSON object in baseDir/events.jsonl.
func readLastJSONL(t *testing.T, baseDir string) (map[string]any, error) {
	t.Helper()
	f, err := os.Open(filepath.Join(baseDir, "events.jsonl"))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var last string
	s := bufio.NewScanner(f)
	for s.Scan() {
		if txt := strings.TrimSpace(s.Text()); txt != "" {
			last = txt
		}
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	if last == "" {
		return nil, errors.New("no lines found")
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(last), &m); err != nil {
		return nil, err
	}
	return m, nil
}

func TestEmitLocalFeatures_HappyPath(t *testing.T) {
	base := t.TempDir()
	t.Setenv("AGT_ARTIFACTS_DIR", base)
	t.Setenv("AGT_CALIBRATION_MODE", "1")
	t.Setenv("AGT_OBSERVE_JSON", "1")

	ctx := telemetry.WithTurnID(context.Background(), "turn-xyz")
	user := "hello  world\nthis is\tgo"

	want := metrics.CountFeatures(user)

	telemetry.EmitLocalFeatures(ctx, user)

	m, err := readLastJSONL(t, base)
	if err != nil {
		t.Fatalf("read last jsonl: %v", err)
	}
	if m["event"] != "local_features" {
		t.Fatalf("event mismatch: %v", m["event"])
	}
	if m["turn_id"] != "turn-xyz" {
		t.Fatalf("turn_id mismatch: %v", m["turn_id"])
	}
	if m["features_version"] != "1" {
		t.Fatalf("features_version mismatch: %v", m["features_version"])
	}

	userMap, ok := m["user"].(map[string]any)
	if !ok {
		t.Fatalf("user field missing or wrong type: %T", m["user"])
	}
	// numbers decode as float64
	if userMap["bytes"] != float64(want.Bytes) ||
		userMap["runes"] != float64(want.Runes) ||
		userMap["words"] != float64(want.Words) ||
		userMap["lines"] != float64(want.Lines) {
		t.Fatalf("user features mismatch: got %#v, want %#v", userMap, want)
	}

	// No raw text leakage (no field named text and no substring of input)
	if _, ok := m["text"]; ok {
		t.Fatalf("unexpected raw text field present")
	}
	raw := strings.ToLower(strings.TrimSpace(user))
	if b, _ := json.Marshal(m); strings.Contains(strings.ToLower(string(b)), raw) && raw != "" {
		t.Fatalf("raw user text leaked into event JSON: %q", raw)
	}
}

func TestEmitLocalFeatures_ObserveOff_NoEvent(t *testing.T) {
	base := t.TempDir()
	t.Setenv("AGT_ARTIFACTS_DIR", base)
	t.Setenv("AGT_CALIBRATION_MODE", "1")
	t.Setenv("AGT_OBSERVE_JSON", "0")

	telemetry.EmitLocalFeatures(context.Background(), "some text")

	if _, err := os.Stat(filepath.Join(base, "events.jsonl")); !os.IsNotExist(err) {
		t.Fatalf("expected no events.jsonl when observe=0, got err=%v", err)
	}
}

func TestEmitLocalFeatures_CalibrationOff_NoEvent(t *testing.T) {
	base := t.TempDir()
	t.Setenv("AGT_ARTIFACTS_DIR", base)
	t.Setenv("AGT_CALIBRATION_MODE", "0")
	t.Setenv("AGT_OBSERVE_JSON", "1")

	telemetry.EmitLocalFeatures(context.Background(), "whatever")

	if _, err := os.Stat(filepath.Join(base, "events.jsonl")); !os.IsNotExist(err) {
		t.Fatalf("expected no events.jsonl when calibration=0, got err=%v", err)
	}
}

func TestEmitLocalFeatures_EmptyInput_Zeros(t *testing.T) {
	base := t.TempDir()
	t.Setenv("AGT_ARTIFACTS_DIR", base)
	t.Setenv("AGT_CALIBRATION_MODE", "1")
	t.Setenv("AGT_OBSERVE_JSON", "1")

	ctx := telemetry.WithTurnID(context.Background(), "turn-empty")
	telemetry.EmitLocalFeatures(ctx, "")

	m, err := readLastJSONL(t, base)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	userMap := m["user"].(map[string]any)
	if userMap["bytes"] != float64(0) || userMap["runes"] != float64(0) || userMap["words"] != float64(0) || userMap["lines"] != float64(0) {
		t.Fatalf("expected all zeros, got %#v", userMap)
	}
}

func TestEmitLocalFeatures_MultibyteAndMultiline(t *testing.T) {
	base := t.TempDir()
	t.Setenv("AGT_ARTIFACTS_DIR", base)
	t.Setenv("AGT_CALIBRATION_MODE", "1")
	t.Setenv("AGT_OBSERVE_JSON", "1")

	ctx := telemetry.WithTurnID(context.Background(), "turn-multi")

	// Multibyte sample
	s1 := "héllö 世界" // bytes=14, runes=8, words=2, lines=1
	telemetry.EmitLocalFeatures(ctx, s1)
	m1, err := readLastJSONL(t, base)
	if err != nil {
		t.Fatalf("read m1: %v", err)
	}
	u1 := m1["user"].(map[string]any)
	if u1["bytes"] != float64(14) || u1["runes"] != float64(8) || u1["words"] != float64(2) || u1["lines"] != float64(1) {
		t.Fatalf("multibyte mismatch: %#v", u1)
	}

	// Multiline sample with trailing newline
	s2 := "a\nb\n" // bytes=4, runes=4, words=2, lines=3
	telemetry.EmitLocalFeatures(ctx, s2)
	m2, err := readLastJSONL(t, base)
	if err != nil {
		t.Fatalf("read m2: %v", err)
	}
	u2 := m2["user"].(map[string]any)
	if u2["bytes"] != float64(4) || u2["runes"] != float64(4) || u2["words"] != float64(2) || u2["lines"] != float64(3) {
		t.Fatalf("multiline mismatch: %#v", u2)
	}
}

func TestEmitLocalFeatures_NoRawTextLeakage(t *testing.T) {
	base := t.TempDir()
	t.Setenv("AGT_ARTIFACTS_DIR", base)
	t.Setenv("AGT_CALIBRATION_MODE", "1")
	t.Setenv("AGT_OBSERVE_JSON", "1")

	ctx := telemetry.WithTurnID(context.Background(), "turn-privacy")
	user := "Foo\u2003Bar\nBaz"

	telemetry.EmitLocalFeatures(ctx, user)

	// Read raw file and ensure the literal user text does not appear.
	b, err := os.ReadFile(filepath.Join(base, "events.jsonl"))
	if err != nil {
		t.Fatalf("read events: %v", err)
	}
	if strings.Contains(string(b), user) && strings.TrimSpace(user) != "" {
		t.Fatalf("raw input text found in events.jsonl")
	}

	// Also assert there's no top-level text fields.
	m, err := readLastJSONL(t, base)
	if err != nil {
		t.Fatalf("read last: %v", err)
	}
	if _, ok := m["text"]; ok {
		t.Fatalf("unexpected text field present in event")
	}
}

func TestEmitLocalFeatures_ArtifactsDirSpaces_AndNewlineTermination(t *testing.T) {
	tmp := t.TempDir()
	base := filepath.Join(tmp, "dir with spaces")
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatalf("mkdir base: %v", err)
	}

	t.Setenv("AGT_ARTIFACTS_DIR", base)
	t.Setenv("AGT_CALIBRATION_MODE", "1")
	t.Setenv("AGT_OBSERVE_JSON", "1")

	ctx := telemetry.WithTurnID(context.Background(), "turn-path")

	telemetry.EmitLocalFeatures(ctx, "one")
	telemetry.EmitLocalFeatures(ctx, "two")

	// File exists with two lines and ends with a newline
	path := filepath.Join(base, "events.jsonl")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("want 2 lines, got %d", len(lines))
	}
	if len(b) == 0 || b[len(b)-1] != '\n' {
		t.Fatalf("expected newline-terminated JSONL file")
	}
}
