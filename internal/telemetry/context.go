package telemetry

import "context"

// turnIDKey is the context key type used to store a turn ID.
type turnIDKey struct{}

// WithTurnID returns a child context that carries the provided turn ID.
// If ctx is nil, context.Background() is used
func WithTurnID(ctx context.Context, id string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, turnIDKey{}, id)
}

// TurnIDFromContext returns the turn ID from ctx, if present.
// Returns "", false if the value is missing or not a non-empty string.
func TurnIDFromContext(ctx context.Context) (string, bool) {
	if ctx == nil {
		return "", false
	}
	v := ctx.Value(turnIDKey{})
	s, ok := v.(string)
	if !ok || s == "" {
		return "", false
	}
	return s, true
}
