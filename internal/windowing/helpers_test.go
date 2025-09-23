package windowing_test

import (
	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/packages/param"
	"github.com/petasbytes/go-agent/internal/windowing"
)

// Text block constructor
func T(text string) anthropic.ContentBlockParamUnion {
	return anthropic.ContentBlockParamUnion{OfText: &anthropic.TextBlockParam{Text: text}}
}

// Tool-use block constructor
func TU(id string) anthropic.ContentBlockParamUnion {
	return anthropic.ContentBlockParamUnion{OfToolUse: &anthropic.ToolUseBlockParam{ID: id}}
}

// Tool-result (no payload), with optional error flag - used by grouping tests where payload length is irrelevant
func TR(id string, isErr bool) anthropic.ContentBlockParamUnion {
	tr := anthropic.ToolResultBlockParam{ToolUseID: id}
	if isErr {
		tr.IsError = param.NewOpt(true)
	}
	return anthropic.ContentBlockParamUnion{OfToolResult: &tr}
}

// Tool-result (string payload) constructor - preferred in counter tests for deterministic sizing
func TRString(id, s string) anthropic.ContentBlockParamUnion {
	return anthropic.NewToolResultBlock(id, s, false)
}

// Tool-result (nested content) constructor - used by counter tests for nested payload handling
func TRNested(id string, nested []anthropic.ContentBlockParamUnion) anthropic.ContentBlockParamUnion {
	// Convert ContentBlockParamUnion to ToolResultBlockParamContentUnion
	content := make([]anthropic.ToolResultBlockParamContentUnion, len(nested))
	for i, block := range nested {
		if textBlock := block.OfText; textBlock != nil {
			content[i] = anthropic.ToolResultBlockParamContentUnion{
				OfText: textBlock,
			}
		}
		// Other block types can be added here when/if needed
	}
	return anthropic.ContentBlockParamUnion{
		OfToolResult: &anthropic.ToolResultBlockParam{ToolUseID: id, Content: content},
	}
}

// Assistant message constructor
func Asst(blocks ...anthropic.ContentBlockParamUnion) anthropic.MessageParam {
	return anthropic.MessageParam{Role: anthropic.MessageParamRoleAssistant, Content: blocks}
}

// User message constructor
func User(blocks ...anthropic.ContentBlockParamUnion) anthropic.MessageParam {
	return anthropic.MessageParam{Role: anthropic.MessageParamRoleUser, Content: blocks}
}

// Intervening returns a message that simply breaks adjacency between
// assistant(tool_use) and the expected next user(tool_result).
func Intervening(text string) anthropic.MessageParam {
	return anthropic.MessageParam{Role: anthropic.MessageParamRoleAssistant, Content: []anthropic.ContentBlockParamUnion{T(text)}}
}

// groupsEqual is a small utility used by grouping tests.
func groupsEqual(got, want []windowing.Group) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i].Kind != want[i].Kind || got[i].Start != want[i].Start || got[i].End != want[i].End {
			return false
		}
	}
	return true
}
