package windowing_test

import (
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/petasbytes/go-agent/internal/windowing"
)

func TestGroupBlocks_Invariants(t *testing.T) {
	tests := []struct {
		name string
		msgs []anthropic.MessageParam
		want []windowing.Group
	}{
		{
			name: "valid pair: one tool",
			msgs: []anthropic.MessageParam{
				Asst(TU("t1")),
				User(TR("t1", false), T("ok")),
			},
			want: []windowing.Group{{Kind: windowing.GroupPair, Start: 0, End: 2}},
		},
		{
			name: "invalid ordering: text before result",
			msgs: []anthropic.MessageParam{
				Asst(TU("t1")),
				User(T("oops"), TR("t1", false)),
			},
			want: []windowing.Group{{Kind: windowing.GroupSingleton, Start: 0, End: 1}, {Kind: windowing.GroupSingleton, Start: 1, End: 2}},
		},
		{
			name: "parallel completeness missing (2 tools)",
			msgs: []anthropic.MessageParam{
				Asst(TU("t1"), TU("t2")),
				User(TR("t1", false)),
			},
			want: []windowing.Group{{Kind: windowing.GroupSingleton, Start: 0, End: 1}, {Kind: windowing.GroupSingleton, Start: 1, End: 2}},
		},
		{
			name: "parallel completeness OK (2 tools) with trailing text",
			msgs: []anthropic.MessageParam{
				Asst(TU("t1"), TU("t2")),
				User(TR("t2", false), TR("t1", false), T("done")),
			},
			want: []windowing.Group{{Kind: windowing.GroupPair, Start: 0, End: 2}},
		},
		{
			name: "intervening message invalidates adjacency",
			msgs: []anthropic.MessageParam{
				Asst(TU("t1")),
				Intervening("note"),
				User(TR("t1", false)),
			},
			want: []windowing.Group{
				{Kind: windowing.GroupSingleton, Start: 0, End: 1},
				{Kind: windowing.GroupSingleton, Start: 1, End: 2},
				{Kind: windowing.GroupSingleton, Start: 2, End: 3},
			},
		},
		{
			name: "error tool_result treated same as non-error",
			msgs: []anthropic.MessageParam{
				Asst(TU("t1")),
				User(TR("t1", true), T("err text")),
			},
			want: []windowing.Group{{Kind: windowing.GroupPair, Start: 0, End: 2}},
		},
		{
			name: "extra results: strict exclusion",
			msgs: []anthropic.MessageParam{
				Asst(TU("t1")),
				User(TR("t1", false), TR("t_extra", false)),
			},
			want: []windowing.Group{{Kind: windowing.GroupSingleton, Start: 0, End: 1}, {Kind: windowing.GroupSingleton, Start: 1, End: 2}},
		},
		{
			name: "assistant with tool_use not followed by user",
			msgs: []anthropic.MessageParam{
				Asst(TU("t1")),
			},
			want: []windowing.Group{{Kind: windowing.GroupSingleton, Start: 0, End: 1}},
		},
		{
			name: "no tools in assistant: both singletons",
			msgs: []anthropic.MessageParam{
				Asst(T("hello")),
				User(T("world")),
			},
			want: []windowing.Group{
				{Kind: windowing.GroupSingleton, Start: 0, End: 1},
				{Kind: windowing.GroupSingleton, Start: 1, End: 2},
			},
		},
		{
			name: "results split by text (invalid ordering)",
			msgs: []anthropic.MessageParam{
				Asst(TU("t1")),
				User(TR("t1", false), T("mid"), TR("t1", false)),
			},
			want: []windowing.Group{
				{Kind: windowing.GroupSingleton, Start: 0, End: 1},
				{Kind: windowing.GroupSingleton, Start: 1, End: 2},
			},
		},
		{
			name: "user text only after tool_use (no results)",
			msgs: []anthropic.MessageParam{
				Asst(TU("t1")),
				User(T("just text")),
			},
			want: []windowing.Group{
				{Kind: windowing.GroupSingleton, Start: 0, End: 1},
				{Kind: windowing.GroupSingleton, Start: 1, End: 2},
			},
		},
		{
			name: "user result has irrelevant ID",
			msgs: []anthropic.MessageParam{
				Asst(TU("t1")),
				User(TR("tX", false)),
			},
			want: []windowing.Group{
				{Kind: windowing.GroupSingleton, Start: 0, End: 1},
				{Kind: windowing.GroupSingleton, Start: 1, End: 2},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := windowing.GroupBlocks(tt.msgs)
			if !groupsEqual(got, tt.want) {
				t.Fatalf("unexpected groups. got=%v want=%v", got, tt.want)
			}
		})
	}
}
