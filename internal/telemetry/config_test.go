package telemetry_test

import (
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"

	"github.com/petasbytes/go-agent/internal/telemetry"
)

// Run TestProbe in a clean env so startup-only telemetry config is deterministic.
// Builds env with PATH + GO_WANT_HELPER_PROCESS, then applies explicit overrides.
func runWithEnv(t *testing.T, env map[string]string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command(os.Args[0], append([]string{"-test.run=TestProbe"}, args...)...)
    // Avoid setting empty AGT_* vars; empty still counts as "set" for LookupEnv.
    base := []string{"GO_WANT_HELPER_PROCESS=1"}
    for _, kv := range os.Environ() {
        if strings.HasPrefix(kv, "PATH=") {
            base = append(base, kv)
            break
        }
    }
    // Apply requested overrides last.
    for k, v := range env {
        base = append(base, k+"="+v)
    }
    cmd.Env = base
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestStartupConfig_Matrix(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want string // encode expected booleans in output: "calib=.. observe=.. persist=.."
	}{
		{"baseline_off", map[string]string{}, "calib=false observe=false persist=false"},
		{"calib_defaults", map[string]string{"AGT_CALIBRATION_MODE": "1"}, "calib=true observe=true persist=true"},
		{"calib_observe_off", map[string]string{"AGT_CALIBRATION_MODE": "1", "AGT_OBSERVE_JSON": "0"}, "calib=true observe=false persist=true"},
		{"calib_persist_off", map[string]string{"AGT_CALIBRATION_MODE": "1", "AGT_PERSIST_API_PAYLOADS": "0"}, "calib=true observe=true persist=false"},
		{"observe_only", map[string]string{"AGT_OBSERVE_JSON": "1"}, "calib=false observe=true persist=false"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := runWithEnv(t, tt.env)
			if err != nil {
				t.Fatalf("subprocess error: %v\n%s", err, got)
			}
			if !containsLine(got, tt.want) {
				t.Fatalf("want line:\n%s\ngot output:\n%s", tt.want, got)
			}
		})
	}
}

// The subprocess probe; in the real test file, import the package under test
// and print the config booleans so parent can assert.
func TestProbe(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	// Print the values for parent process assertion
	fmt.Printf(
		"calib=%v observe=%v persist=%v\n",
		telemetry.CalibrationModeEnabled(),
		telemetry.ObserveEnabled(),
		telemetry.PersistPayloadsEnabled(),
	)
}

// containsLine reports whether output has a line exactly equal to want.
func containsLine(output, want string) bool {
	return slices.Contains(strings.Split(output, "\n"), want)
}
