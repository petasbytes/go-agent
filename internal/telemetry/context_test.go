package telemetry_test

import (
	"context"
	"testing"
	"time"

	"github.com/petasbytes/go-agent/internal/telemetry"
)

func TestTurnID_RoundTrip(t *testing.T) {
	ctx := telemetry.WithTurnID(context.Background(), "turn-123")
	got, ok := telemetry.TurnIDFromContext(ctx)
	if !ok || got != "turn-123" {
		t.Fatalf("want turn-123,true; got %q,%v", got, ok)
	}
}

func TestTurnID_NilParent(t *testing.T) {
	ctx := telemetry.WithTurnID(context.Background(), "t1")
	got, ok := telemetry.TurnIDFromContext(ctx)
	if !ok || got != "t1" {
		t.Fatalf("want t1,true; got %q,%v", got, ok)
	}
}

func TestTurnID_EmptyIDRejectedOnRead(t *testing.T) {
	ctx := telemetry.WithTurnID(context.Background(), "")
	got, ok := telemetry.TurnIDFromContext(ctx)
	if ok || got != "" {
		t.Fatalf("want empty,false; got %q,%v", got, ok)
	}
}

func TestTurnID_ParentCancellationPropagates(t *testing.T) {
	parent, cancel := context.WithCancel(context.Background())
	defer cancel()

	child := telemetry.WithTurnID(parent, "t1")

	// Cancel the parent and ensure child's Done is closed promptly.
	cancel()

	select {
	case <-child.Done():
		// ok
	case <-time.After(100 * time.Millisecond):
		t.Fatal("child context did not observe parent cancellation")
	}
}

func TestTurnID_LastWriteWins(t *testing.T) {
	ctx1 := telemetry.WithTurnID(context.Background(), "t1")
	ctx2 := telemetry.WithTurnID(ctx1, "t2")

	got, ok := telemetry.TurnIDFromContext(ctx2)
	if !ok || got != "t2" {
		t.Fatalf("want t2,true; got %q,%v", got, ok)
	}
}

func TestTurnID_UnrelatedValuesUnaffected(t *testing.T) {
	type otherKey struct{}
	parent := context.WithValue(context.Background(), otherKey{}, 123)

	child := telemetry.WithTurnID(parent, "t1")

	// Unrelated value should still be accessible from child.
	v := child.Value(otherKey{})
	if v != 123 {
		t.Fatalf("want unrelated value 123; got %#v", v)
	}

	// And turn ID remains intact.
	got, ok := telemetry.TurnIDFromContext(child)
	if !ok || got != "t1" {
		t.Fatalf("want t1,true; got %q,%v", got, ok)
	}
}

func TestTurnID_MissingValue(t *testing.T) {
	got, ok := telemetry.TurnIDFromContext(context.Background())
	if ok || got != "" {
		t.Fatalf("want empty,false; got %q,%v", got, ok)
	}
}

func TestTurnID_NilCtxOnRead(t *testing.T) {
	got, ok := telemetry.TurnIDFromContext(context.Background())
	if ok || got != "" {
		t.Fatalf("want empty,false; got %q,%v", got, ok)
	}
}
