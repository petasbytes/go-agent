package windowing

import (
	"unicode/utf8"

	"github.com/anthropics/anthropic-sdk-go"
)

// TokenCounter estimates input-token cost for messages or groups.
type TokenCounter interface {
	CountMessage(m anthropic.MessageParam) int
	CountGroup(g Group, all []anthropic.MessageParam) int
}

// HeuristicCounter is the current default deterministic estimator.
// Rules:
// - text blocks: rune count of TextBlockParam.Text
// - tool_result blocks:
//   - nested ([]anthropic.ContentBlockParamUnion): sum nested text runes
//   - non-nested (e.g. string): count runes of the string representation
//     Add a small per-block overhead to account for minimal formatting.
type HeuristicCounter struct{}

// Fixed per-block overhead for deterministic counts; changing this requires updating the guard test.
const blockOverhead = 4

func (HeuristicCounter) CountMessage(m anthropic.MessageParam) int {
	total := 0
	for _, blk := range m.Content {
		total += countBlock(blk)
	}
	return total
}

func (h HeuristicCounter) CountGroup(g Group, all []anthropic.MessageParam) int {
	total := 0
	for i := g.Start; i < g.End && i < len(all); i++ {
		total += h.CountMessage(all[i])
	}
	return total
}

// Helpers

func countBlock(blk anthropic.ContentBlockParamUnion) int {
	// test block
	if tb := blk.OfText; tb != nil {
		return utf8.RuneCountInString(tb.Text) + blockOverhead
	}

	// tool_result block
	if tr := blk.OfToolResult; tr != nil {
		// Handle nested content
		if nested, ok := any(tr.Content).([]anthropic.ToolResultBlockParamContentUnion); ok {
			subtotal := 0
			for _, nb := range nested {
				if nt := nb.OfText; nt != nil {
					subtotal += utf8.RuneCountInString(nt.Text)
				}
				// Non-text nested blocks contribute only via parent overhead.
			}
			return subtotal + blockOverhead
		}
		// Not-nested (use string representation)
		if s, ok := any(tr.Content).(string); ok {
			return utf8.RuneCountInString(s) + blockOverhead
		}

		// Fallback: unsupported non-nested tool_result payload - count overhead only (logs when verbose).
		vlogf("counter: unsupported_tool_result_payload type=%T using=overhead_only", tr.Content)
		return blockOverhead
	}

	// Default for non-text/non-tool_result blocks (thinking, tool_use, images/documents in user messages, etc.) -
	// count overhead only in this minimal heuristic. Can be extended later if required.
	return blockOverhead
}
