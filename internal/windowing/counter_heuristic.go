package windowing

import (
	"github.com/anthropics/anthropic-sdk-go"
)

// TokenCounter estimates input-token cost for messages or groups.
type TokenCounter interface {
	CountMessage(m anthropic.MessageParam) int
	CountGroup(g Group, all []anthropic.MessageParam) int
}

// HeuristicCounter is the current default counter.
type HeuristicCounter struct{}

const blockOverhead = 4

func (HeuristicCounter) CountMessage(m anthropic.MessageParam) int {
	// For now, use a simple, deterministic estimate based on number of content blocks.
	// Detailed text/tool_result accounting will be added in a later milestone.
	return len(m.Content) * blockOverhead
}

func (h HeuristicCounter) CountGroup(g Group, all []anthropic.MessageParam) int {
	total := 0
	for i := g.Start; i < g.End && i < len(all); i++ {
		total += h.CountMessage(all[i])
	}
	return total
}
