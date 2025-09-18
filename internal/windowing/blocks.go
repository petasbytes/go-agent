package windowing

import "github.com/anthropics/anthropic-sdk-go"

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

// GroupBlocks groups messages into atomic units.
func GroupBlocks(msgs []anthropic.MessageParam) []Group {
	groups := make([]Group, 0, len(msgs))
	for i := range msgs {
		groups = append(groups, Group{Kind: GroupSingleton, Start: i, End: i + 1})
	}
	return groups
}
