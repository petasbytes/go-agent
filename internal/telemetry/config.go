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
func CalibrationModeEnabled() bool { return calibrationModeEnabled }

// ObserveEnabled resports whether JSONL emission was enabled at starup, considering calbiration defaults.
func ObserveEnabled() bool {
	// Preserve startup-evaluated default, but allow tests to enable mid-run via env override.
	if os.Getenv("AGT_OBSERVE_JSON") == "1" {
		return true
	}
	return observeEnabled
}

// PersistPayloadsEnabled reports whether request and response payload persistence was enabled at startup.
func PersistPayloadsEnabled() bool {
	// Preserve startup-evaluated default, but allow tests to enable mid-run via env override.
	if os.Getenv("AGT_PERSIST_API_PAYLOADS") == "1" {
		return true
	}
	return persistPayloadsEnabled
}
