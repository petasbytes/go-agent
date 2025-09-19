package windowing

import (
	"fmt"
	"os"

	"github.com/anthropics/anthropic-sdk-go"
)

// GroupKind denotes the atomic unit type when preparing a send window.
type GroupKind int

const (
	GroupSingleton GroupKind = iota
	GroupPair
)

// Group describes a contiguous span of messages [Start, End) in the original slice.
// Kind indicates whether it is a singleton or a validated pair.
type Group struct {
	Kind  GroupKind
	Start int // inclusive index into msgs
	End   int // exclusive index into msgs
}

// GroupBlocks groups messages into atomic units that preserve tool-use pairs.
// Invariants:
// - A pair is exactly two adjacent messages: assistant(tool_use+...) then user(tool_result...).
// - In the user message, all tool_result blocks must come first; text (if any) comes after.
// - Parallel completeness: all tool_use ids in the assistant must appear as tool_result
// ids in the following user message's leading tool_result segment.
// - tool_result blocks with is_error=true are treated the same for grouping.
func GroupBlocks(msgs []anthropic.MessageParam) []Group {
	groups := make([]Group, 0, len(msgs))
	for i := 0; i < len(msgs); {
		m := msgs[i]
		if isAssistant(m) {
			// Detect tool_use blocks and collect IDs.
			useIDs := collectToolUseIDs(m)
			if len(useIDs) > 0 {
				// Check adjacency and user validation.
				if i+1 < len(msgs) && isUser(msgs[i+1]) {
					valid, resultIDs := leadingToolResultIDsAndOrderingValid(msgs[i+1])
					if valid && coversAll(resultIDs, useIDs) && noExtraResults(resultIDs, useIDs) {
						groups = append(groups, Group{Kind: GroupPair, Start: i, End: i + 2})
						i += 2
						continue
					}
					// Reason-coded verbose logs (behind AGT_VERBOSE_WINDOW_LOGS)
					reason := ""
					switch {
					case !valid:
						reason = "ordering_invalid"
					case !coversAll(resultIDs, useIDs):
						reason = "missing_results"
					case !noExtraResults(resultIDs, useIDs):
						reason = "extra_results"
					default:
						reason = "unknown"
					}
					vlogf("exclude pair: reason=%s idx=%d", reason, i)
				} else {
					vlogf("exclude pair: reason=not_followed_by_user idx=%d", i)
				}
			}
		}
		// Fallback: singleton
		groups = append(groups, Group{Kind: GroupSingleton, Start: i, End: i + 1})
		i++
	}
	return groups
}

// Helpers

func isAssistant(m anthropic.MessageParam) bool {
	return m.Role == anthropic.MessageParamRoleAssistant
}

func isUser(m anthropic.MessageParam) bool {
	return m.Role == anthropic.MessageParamRoleUser
}

// collectToolUseIDs returns the set of tool_use ids present in an assistant message.
func collectToolUseIDs(m anthropic.MessageParam) map[string]struct{} {
	ids := make(map[string]struct{})
	for _, blk := range m.Content {
		if tu := blk.OfToolUse; tu != nil {
			// ID is a required string field in ToolUseBlockParam
			if tu.ID != "" {
				ids[tu.ID] = struct{}{}
			}
		}
	}
	return ids
}

// leadingToolResultIDsAndOrderingValid inspects a user message and returns:
// - valid=false if any non-tool_result block appears before a tool_result
// - resultIDs: the ids of tool_result blocks in the leading tool_result segment.
// Text after the leading tool_result segment is allowed and ignored for id collection.
func leadingToolResultIDsAndOrderingValid(m anthropic.MessageParam) (valid bool, resultIDs map[string]struct{}) {
	resultIDs = make(map[string]struct{})
	seenNonResult := false
	for _, blk := range m.Content {
		if tr := blk.OfToolResult; tr != nil {
			if seenNonResult {
				// tool_result after non-result: invalid ordering
				return false, resultIDs
			}
			// ToolUseID is a required string field in ToolResultBlockParam
			if tr.ToolUseID != "" {
				resultIDs[tr.ToolUseID] = struct{}{}
			}
			continue
		}
		// once we see first non-tool_result block, we mark the boundary
		seenNonResult = true
	}
	return true, resultIDs
}

// coversAll checks that every id in required is present in have.
func coversAll(have, required map[string]struct{}) bool {
	for id := range required {
		if _, ok := have[id]; !ok {
			return false
		}
	}
	return true
}

// noExtraResults optionally enforces that the user didn't return extra results
// that do not correspond to any tool_use in the assistant turn. Keeping this
// strict avoids mismatches and simplifies downstream logic.
func noExtraResults(have, allowed map[string]struct{}) bool {
	for id := range have {
		if _, ok := allowed[id]; !ok {
			return false
		}
	}
	return true
}

// minimal verbose logging when AGT_VERBOSE_WINDOW_LOGS=1
var verbose = os.Getenv("AGT_VERBOSE_WINDOW_LOGS") == "1"

func vlogf(format string, args ...any) {
	if verbose {
		fmt.Printf("[windowing] "+format+"\n", args...)
	}
}
