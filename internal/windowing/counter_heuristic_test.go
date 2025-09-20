// Package windowing_test contains tests for the heuristic token counter.
// Tests focus on rune counting correctness, tool result payload handling,
// and deterministic overhead application.
package windowing_test

import (
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/petasbytes/go-agent/internal/windowing"
)

func TRString(id, s string) anthropic.ContentBlockParamUnion {
	return anthropic.NewToolResultBlock(id, s, false)
}

func TRNested(id string, nested []anthropic.ContentBlockParamUnion) anthropic.ContentBlockParamUnion {
	// Convert ContentBlockParamUnion to ToolResultBlockParamContentUnion
	content := make([]anthropic.ToolResultBlockParamContentUnion, len(nested))
	for i, block := range nested {
		if textBlock := block.OfText; textBlock != nil {
			content[i] = anthropic.ToolResultBlockParamContentUnion{
				OfText: textBlock,
			}
		}
		// Add other block types as needed
	}

	return anthropic.ContentBlockParamUnion{
		OfToolResult: &anthropic.ToolResultBlockParam{
			ToolUseID: id,
			Content:   content,
		},
	}
}

func TestHeuristicCounter_TextBlocks_CountsRunes(t *testing.T) {
	h := windowing.HeuristicCounter{}
	// ASCII + multibyte (emoji)
	msg := User(T("hello"), T("👍"))
	got := h.CountMessage(msg)
	// Derive per-block overhead from an empty text block (0 runes => result equals overhead)
	overhead := h.CountMessage(User(T("")))
	// "hello" = 5 runes, "👍" = 1 rune; 2 blocks overhead
	want := (5 + 1) + 2*overhead
	if got != want {
		t.Fatalf("got=%d want=%d", got, want)
	}
}

func TestHeuristicCounter_ToolResult_StringPayload(t *testing.T) {
	h := windowing.HeuristicCounter{}
	payload := "abcdef" // 6 runes
	msg := User(TRString("t1", payload))
	got := h.CountMessage(msg)
	overhead := h.CountMessage(User(T("")))
	want := 6 + overhead
	if got != want {
		t.Fatalf("got=%d want=%d", got, want)
	}
}

func TestHeuristicCounter_ToolResult_NestedTextPayload(t *testing.T) {
	h := windowing.HeuristicCounter{}
	nested := []anthropic.ContentBlockParamUnion{
		T("hi"), // 2
		T("世界"), // 2 runes (Chinese chars)
	}
	msg := User(TRNested("t1", nested))
	got := h.CountMessage(msg)
	overhead := h.CountMessage(User(T("")))
	want := (2 + 2) + overhead
	if got != want {
		t.Fatalf("got=%d want=%d", got, want)
	}
}

func TestHeuristicCounter_CountGroup_SumsMessages(t *testing.T) {
	h := windowing.HeuristicCounter{}
	msgs := []anthropic.MessageParam{
		User(T("a")),                // 1 + overhead
		Asst(T("b"), T("c")),        // 1+1 + 2*overhead
		User(TRString("t1", "xyz")), // 3 + overhead
	}
	groups := []windowing.Group{{Kind: windowing.GroupSingleton, Start: 0, End: 1}, {Kind: windowing.GroupSingleton, Start: 1, End: 2}, {Kind: windowing.GroupSingleton, Start: 2, End: 3}}

	total := 0
	for _, g := range groups {
		total += h.CountGroup(g, msgs)
	}

	overhead := h.CountMessage(User(T("")))
	want := (1 + overhead) + (1 + 1 + 2*overhead) + (3 + overhead)
	if total != want {
		t.Fatalf("got=%d want=%d", total, want)
	}
}
