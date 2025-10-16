package telemetry

import (
	"context"

	"github.com/petasbytes/go-agent/internal/metrics"
)

func EmitLocalFeatures(ctx context.Context, user string) {
	if !(CalibrationModeEnabled() && ObserveEnabled()) {
		return
	}
	turnID, _ := TurnIDFromContext(ctx)
	f := metrics.CountFeatures(user)
	Emit("local_features", map[string]any{
		"turn_id":          turnID,
		"features_version": "1",
		"user": map[string]any{
			"bytes": f.Bytes,
			"runes": f.Runes,
			"words": f.Words,
			"lines": f.Lines,
		},
	})
}
