package windowing_test

import (
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/petasbytes/go-agent/internal/windowing"
)

func TestPrepareSendWindow_BudgetRespected_OrderPreserved(t *testing.T) {
	// Oldest -> newest
	msgs := []anthropic.MessageParam{
		User(T("old")), // G0: 3 + 4 = 7
		Asst(TU("a")),
		User(TRString("a", "r")),
		User(T("tail")),
	}
	budget := 17 // G2(8)  G1(9) = 17

	window, stats := windowing.PrepareSendWindow(msgs, budget, windowing.HeuristicCounter{})

	if stats.Budget != budget || stats.Total != 17 || stats.IncludedGroups != 2 || stats.OverBudgetNewest {
		t.Fatalf("unexpected stats: %+v", stats)
	}

	if len(window) != 3 { // expect msgs[1:]
		t.Fatalf("unexpected window length: got %d want=3", len(window))
	}
	if window[0].Role != anthropic.MessageParamRoleAssistant || window[1].Role != anthropic.MessageParamRoleUser || window[2].Role != anthropic.MessageParamRoleUser {
		t.Fatalf("unexpected roles order in window: %+v", []anthropic.MessageParam{window[0], window[1], window[2]})
	}
}

func TestPrepareSendWindow_NewestGroupOverBudget(t *testing.T) {
	msgs := []anthropic.MessageParam{
		User(T("old")),                // G0: 7
		Asst(TU("a")),                 // G1 part: 4
		User(TRString("a", "xxxxxx")), // G1 part: 6 + 4 = 10 => G1 total 14 (newest)
	}
	budget := 10 // less than newest group cost (14)

	window, stats := windowing.PrepareSendWindow(msgs, budget, windowing.HeuristicCounter{})

	if len(window) != 0 {
		t.Fatalf("expected empty window; got=%d", len(window))
	}
	if !stats.OverBudgetNewest || stats.IncludedGroups != 0 || stats.SkippedGroups == 0 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
}

func TestPrepareSendWindow_NoCapacityBudget_WithGroups(t *testing.T) {
	msgs := []anthropic.MessageParam{
		User(T("x")), // at least one group
	}
	window, stats := windowing.PrepareSendWindow(msgs, 0, windowing.HeuristicCounter{})

	if len(window) != 0 || !stats.OverBudgetNewest || stats.SkippedGroups != 1 || stats.IncludedGroups != 0 {
		t.Fatalf("unpexpected stats: %+v", stats)
	}
}

func TestPrepareSendWindow_EmptyMsgs(t *testing.T) {
	window, stats := windowing.PrepareSendWindow(nil, 123, windowing.HeuristicCounter{})
	if window != nil || stats.Budget != 123 || stats.Total != 0 || stats.OverBudgetNewest {
		t.Fatalf("unexpected result: window=%v stats=%+v", window, stats)
	}
}

func TestPrepareSendWindow_AllFitIncludingOldest(t *testing.T) {
	// Groups (oldest -> newest):
	// G0: user("oldest") => cost = len("oldest") + overhead = 6 + 4 = 10
	// G1: user("mid") => 3 + 4 = 7
	// G2: user("new") => 3 + 4 = 7
	// Total expected cost = 24
	msgs := []anthropic.MessageParam{
		User(T("oldest")), // G0
		User(T("mid")),    // G1
		User(T("new")),    // G2 (newest)
	}

	counter := windowing.HeuristicCounter{}

	// Budget allows all three groups
	budget := 24
	window, stats := windowing.PrepareSendWindow(msgs, budget, counter)

	if stats.Budget != budget {
		t.Fatalf("Budget echo mismatch: got=%d want=%d", stats.Budget, budget)
	}
	if stats.OverBudgetNewest {
		t.Fatalf("unexpected OverBudgetNewest")
	}
	if stats.IncludedGroups != 3 || stats.SkippedGroups != 0 {
		t.Fatalf("IncludedGroups/SkippedGroups mismatch: got inc=%d skip=%d", stats.IncludedGroups, stats.SkippedGroups)
	}

	// Expect full window in same order (oldest->newest)
	if len(window) != len(msgs) {
		t.Fatalf("window size: got=%d want=%d", len(window), len(msgs))
	}
	for i := range msgs {
		if window[i].Role != msgs[i].Role {
			t.Fatalf("role mismatch at %d: got=%v want=%v", i, window[i].Role, msgs[i].Role)
		}
	}
}

func TestPrepareSendWindow_ExactlyOneOlderAlsoFits(t *testing.T) {
	// Groups (oldest -> newest):
	// G0: user("a") => 1 + 4 = 5
	// G1: user("bbbb") => 4 + 4 = 8
	// G2: user("cc") => 2 + 4 = 6 (newest)
	// Budget = 14 => include newest (6) + next older (8) = 14; stop before adding oldest (would be 19)
	msgs := []anthropic.MessageParam{
		User(T("a")),    // G0
		User(T("bbbb")), // G1
		User(T("cc")),   // G2 (newest)
	}

	counter := windowing.HeuristicCounter{}

	budget := 14
	window, stats := windowing.PrepareSendWindow(msgs, budget, counter)

	if stats.Budget != budget {
		t.Fatalf("Budget echo mismatch: got=%d want=%d", stats.Budget, budget)
	}
	if stats.OverBudgetNewest {
		t.Fatalf("unexpected OverBudgetNewest")
	}
	if stats.IncludedGroups != 2 || stats.SkippedGroups != 1 {
		t.Fatalf("IncludedGroups/SkippedGroups mismatch: got inc=%d skip=%d", stats.IncludedGroups, stats.SkippedGroups)
	}

	// Expect window to be msgs[1:] i.e., keep G1 and G2 in order (assistant/user roles preserved if present)
	if len(window) != 2 {
		t.Fatalf("window size: got=%d want=2", len(window))
	}

	// Roles should match the corresponding positions
	if window[0].Role != msgs[1].Role || window[1].Role != msgs[2].Role {
		t.Fatalf("role order mismatch: got=[%v,%v] want=[%v,%v]", window[0].Role, window[1].Role, msgs[1].Role, msgs[2].Role)
	}

	// Verify total cost equals budget (6 + 8)
	gotCost := 0
	for _, m := range window {
		gotCost += counter.CountMessage(m)
	}
	if gotCost != 14 {
		t.Fatalf("total cost mismatch: got=%d want=14", gotCost)
	}
}
