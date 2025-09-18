package windowing

import "github.com/anthropics/anthropic-sdk-go"

// Stats summarises windowing outcomes (to be extended later).
type Stats struct {
	Total            int // estimated tokens included
	Budget           int // configured budget used for the call
	IncludedGroups   int
	SkippedGroups    int
	OverBudgetNewest bool // newest single group exceeds budget
}

// PrepareSendWindow returns a slice of messages within a budget without splitting groups.
// For now, just pass-through entire conversation and compute a trivial estimate.
// Does not yet change runtime behaviour (runner integration to be added later).
func PrepareSendWindow(msgs []anthropic.MessageParam, budget int, c TokenCounter) ([]anthropic.MessageParam, Stats) {
	// For now, pass-through window; compute simple stats so callers can log if desired.
	groups := GroupBlocks(msgs)
	est := 0
	included := 0
	for _, g := range groups {
		est += c.CountGroup(g, msgs)
		included++
	}
	return msgs, Stats{
		Total:          est,
		Budget:         budget,
		IncludedGroups: included,
		SkippedGroups:  0,
	}
}
