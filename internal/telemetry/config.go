package telemetry

import (
	"os"
)

var (
	calibrationModeEnabled bool
	observeEnabled         bool
	persistPayloadsEnabled bool
)

func init() {
	// Read once at process start. Mid-run environment changes have no effect.
	calibrationModeEnabled = os.Getenv("AGT_CALIBRATION_MODE") == "1"

	// Observe: default to 1 when calibration=1 and AGT_OBSERVE_JSON is unset; honour explicit 0/1.
	if v, ok := os.LookupEnv("AGT_OBSERVE_JSON"); ok {
		observeEnabled = (v == "1")
	} else {
		observeEnabled = calibrationModeEnabled
	}

	// Persist payloads: default to 1 when calibration=1 and AGT_PERSIST_API_PAYLOADS is unset; honour explicit 0/1.
	if v, ok := os.LookupEnv("AGT_PERSIST_API_PAYLOADS"); ok {
		persistPayloadsEnabled = (v == "1")
	} else {
		persistPayloadsEnabled = calibrationModeEnabled
	}
}

// CalibrationModeEnabled reports whether calibration mode was enabled at startup.
func CalibrationModeEnabled() bool {
	// Allow explicit mid-run overrides in tests: "1" -> true, "0" -> false.
	if v, ok := os.LookupEnv("AGT_CALIBRATION_MODE"); ok {
		if v == "1" {
			return true
		}
		if v == "0" {
			return false
		}
	}
	return calibrationModeEnabled
}

// ObserveEnabled reports whether JSONL emission was enabled at startup, considering calibration defaults.
func ObserveEnabled() bool {
	// Allow explicit mid-run overrides in tests: "1" -> true, "0" -> false.
	if v, ok := os.LookupEnv("AGT_OBSERVE_JSON"); ok {
		if v == "1" {
			return true
		}
		if v == "0" {
			return false
		}
	}
	return observeEnabled
}

// PersistPayloadsEnabled reports whether request and response payload persistence was enabled at startup.
func PersistPayloadsEnabled() bool {
	// Allow explicit mid-run overrides in tests: "1" -> true, "0" -> false.
	if v, ok := os.LookupEnv("AGT_PERSIST_API_PAYLOADS"); ok {
		if v == "1" {
			return true
		}
		if v == "0" {
			return false
		}
	}
	return persistPayloadsEnabled
}
