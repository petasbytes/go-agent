package runner_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/petasbytes/go-agent/internal/provider"
	"github.com/petasbytes/go-agent/internal/runner"
	"github.com/petasbytes/go-agent/internal/telemetry"
	"github.com/petasbytes/go-agent/tools"
)

type capture struct {
	method string
	url    string
	body   []byte
}

// Helper: change to temp dir for duration of test and restore after.
func chdirTemp(t *testing.T) string {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	return tmp
}

// Helper: capture stdout for duration of function f.
func captureStdout(t *testing.T, f func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	defer func() { os.Stdout = old }()

	done := make(chan string)
	go func() {
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		done <- buf.String()
	}()

	f()
	_ = w.Close()
	out := <-done
	return out
}

// Helper: read .agent/events/jsonl lines; returns slice of non-empty lines (or empty if file missing)
func readEventLines(t *testing.T) []string {
	t.Helper()
	b, err := os.ReadFile(".agent/events.jsonl")
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("read events.jsonl: %v", err)
	}
	var out []string
	for _, ln := range strings.Split(string(b), "\n") {
		if s := strings.TrimSpace(ln); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func TestRunner_IncludesNewestToolPairOnly_WhenBudgetFitsPair(t *testing.T) {
	// Budget fits the newest pair (assistant tool_use + user tool_result)
	// and excludes the older standalone user message.
	t.Setenv("AGT_TOKEN_BUDGET", "10")

	capReq := &capture{}
	fake := &fakeTransport{respStatus: 200, respBody: []byte(`{"content": [], "role":"assistant"}`), captured: capReq}
	cli := newClientWithTransport(fake)
	r := runner.New(cli, tools.Registry())

	// Conversation: oldest -> newest
	// 1) user("old")
	// 2) assistant(tool_use id="a")
	// 3) user(tool_result tool_use_id="a")
	toolUse := anthropic.ToolUseBlockParam{
		Type: "tool_use",
		ID:   "a",
		Name: "dummy_tool", // input omitted; not needed for this pairing test
	}
	toolRes := anthropic.ToolResultBlockParam{
		Type:      "tool_result",
		ToolUseID: "a",
	}

	conv := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock("old")),
		anthropic.NewAssistantMessage(anthropic.ContentBlockParamUnion{OfToolUse: &toolUse}),
		anthropic.NewUserMessage(anthropic.ContentBlockParamUnion{OfToolResult: &toolRes}),
	}

	_, _, err := r.RunOneStep(context.Background(), provider.DefaultModel, conv)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if capReq.body == nil {
		t.Fatal("no request captured")
	}

	// Decode request and assert only the newest pair was sent.
	type contentItem struct {
		Type      string          `json:"type"`
		Text      string          `json:"text,omitempty"`
		ID        string          `json:"id,omitempty"`
		Name      string          `json:"name,omitempty"`
		Input     json.RawMessage `json:"input,omitempty"`
		ToolUseID string          `json:"tool_use_id,omitempty"`
		IsError   bool            `json:"is_error,omitempty"`
	}
	type reqBodyPair struct {
		Messages []struct {
			Role    string        `json:"role"`
			Content []contentItem `json:"content"`
		} `json:"messages"`
	}

	var rb reqBodyPair
	if err := json.Unmarshal(capReq.body, &rb); err != nil {
		t.Fatalf("unmarshal body: %v\nbody=%s", err, string(capReq.body))
	}

	if len(rb.Messages) != 2 {
		t.Fatalf("expected exactly the newest pair (2 messages), got %d", len(rb.Messages))
	}
	// Assistant tool_use (id "a")
	if rb.Messages[0].Role != "assistant" || len(rb.Messages[0].Content) == 0 || rb.Messages[0].Content[0].Type != "tool_use" || rb.Messages[0].Content[0].ID != "a" {
		t.Fatalf("unexpected first message (assistant tool_use): %+v", rb.Messages[0])
	}
	// User tool_result (tool_use_id "a")
	if rb.Messages[1].Role != "user" || len(rb.Messages[1].Content) == 0 || rb.Messages[1].Content[0].Type != "tool_result" || rb.Messages[1].Content[0].ToolUseID != "a" {
		t.Fatalf("unexpected second message (user tool_result): %+v", rb.Messages[1])
	}
}

type fakeTransport struct {
	respStatus int
	respBody   []byte
	captured   *capture
}

func (f *fakeTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	b, _ := io.ReadAll(req.Body)
	_ = req.Body.Close()
	if f.captured != nil {
		f.captured.method = req.Method
		f.captured.url = req.URL.String()
		f.captured.body = b
	}
	resp := &http.Response{
		StatusCode: f.respStatus,
		Body:       io.NopCloser(bytes.NewReader(f.respBody)),
		Header:     make(http.Header),
	}
	resp.Header.Set("Content-Type", "application/json")
	return resp, nil
}

func newClientWithTransport(rt http.RoundTripper) *anthropic.Client {
	c := anthropic.NewClient(
		option.WithHTTPClient(&http.Client{Transport: rt}),
		option.WithAPIKey("test-key"),
		// Base URL is irrelevant since transport intercepts
	)
	return &c
}

type reqBody struct {
	Messages []struct {
		Role    string `json:"role"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text,omitempty"`
		} `json:"content"`
	} `json:"messages"`
}

func TestRunner_MissingBudget_ReturnsError(t *testing.T) {
	t.Setenv("AGT_TOKEN_BUDGET", "")
	cli := newClientWithTransport(&fakeTransport{respStatus: 200, respBody: []byte(`{"content":[],"role":"assistant"}`)})
	r := runner.New(cli, tools.Registry())
	_, _, err := r.RunOneStep(context.Background(), provider.DefaultModel, nil)
	if err == nil || !strings.Contains(err.Error(), "AGT_TOKEN_BUDGET not set") {
		t.Fatalf("expected env error, got %v", err)
	}
}

func TestRunner_InvalidBudget_ReturnsError(t *testing.T) {
	t.Setenv("AGT_TOKEN_BUDGET", "abc")
	cli := newClientWithTransport(&fakeTransport{respStatus: 200, respBody: []byte(`{"content":[],"role":"assistant"}`)})
	r := runner.New(cli, tools.Registry())
	_, _, err := r.RunOneStep(context.Background(), provider.DefaultModel, nil)
	if err == nil || !strings.Contains(err.Error(), "invalid AGT_TOKEN_BUDGET") {
		t.Fatalf("expected parse error, got %v", err)
	}
}

func TestRunner_OverBudgetNewest_ReturnsError_NoHTTP(t *testing.T) {
	// Guard: newest group over budget returns error and makes no HTTP call.
	t.Setenv("AGT_TOKEN_BUDGET", "1")
	capReq := &capture{}
	fake := &fakeTransport{respStatus: 200, respBody: []byte(`{"content":[],"role":"assistant"}`), captured: capReq}
	cli := newClientWithTransport(fake)
	r := runner.New(cli, tools.Registry())
	conv := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock("hello")),
	}
	_, _, err := r.RunOneStep(context.Background(), provider.DefaultModel, conv)
	if err == nil || !strings.Contains(err.Error(), "newest group exceeds AGT_TOKEN_BUDGET") {
		t.Fatalf("expected over-budget newest error, got %v", err)
	}
	if capReq.body != nil {
		t.Fatalf("expected no HTTP call when over-budget newest; got body len=%d", len(capReq.body))
	}
}

func TestRunner_SendsPreparedWindowSubset(t *testing.T) {
	// Sends only the prepared window (last message), not the full conversation.
	t.Setenv("AGT_TOKEN_BUDGET", "10")
	capReq := &capture{}
	fake := &fakeTransport{respStatus: 200, respBody: []byte(`{"content":[],"role":"assistant"}`), captured: capReq}
	cli := newClientWithTransport(fake)
	r := runner.New(cli, tools.Registry())
	conv := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock("abc")),
		anthropic.NewUserMessage(anthropic.NewTextBlock("defgh")),
	}
	_, _, err := r.RunOneStep(context.Background(), provider.DefaultModel, conv)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if capReq.body == nil {
		t.Fatal("no request captured")
	}
	var rb reqBody
	if err := json.Unmarshal(capReq.body, &rb); err != nil {
		t.Fatalf("unmarshal body: %v\nbody=%s", err, string(capReq.body))
	}
	if len(rb.Messages) != 1 {
		t.Fatalf("expected 1 message in prepared window, got %d", len(rb.Messages))
	}
	if rb.Messages[0].Role != "user" || len(rb.Messages[0].Content) == 0 || rb.Messages[0].Content[0].Text != "defgh" {
		t.Fatalf("unpexpected prepared window payload: %+v", rb.Messages[0])
	}
}

func TestRunner_ToolUse_ExecutesToolAndReturnsResults(t *testing.T) {
	t.Setenv("AGT_TOKEN_BUDGET", "1000")
	// Fake provider returns a tool_use; runner executes tool and returns tool_result.
	resp := `{
	"role": "assistant",
	"content": [{"type": "tool_use", "id": "t1", "name": "list_files", "input": {"path": "."}}]
	}`
	fake := &fakeTransport{respStatus: 200, respBody: []byte(resp), captured: &capture{}}
	cli := newClientWithTransport(fake)
	r := runner.New(cli, tools.Registry())
	conv := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock("please list files")),
	}
	msg, toolResults, err := r.RunOneStep(context.Background(), provider.DefaultModel, conv)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if msg == nil {
		t.Fatal("nil message returned")
	}
	if len(toolResults) == 0 {
		t.Fatal("expected at least one tool_result from execTool")
	}
}

func TestRunner_WindowPrepared_JSONL_HappyPath_SingleEmission(t *testing.T) {
	t.Setenv("AGT_TOKEN_BUDGET", "10")
	t.Setenv("AGT_OBSERVE_JSON", "1")
	_ = chdirTemp(t)

	capReq := &capture{}
	fake := &fakeTransport{respStatus: 200, respBody: []byte(`{"content":[],"role":"assistant"}`), captured: capReq}
	cli := newClientWithTransport(fake)
	r := runner.New(cli, tools.Registry())

	// Two short user messages; newest should fit within budget.
	conv := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock("hi")),
		anthropic.NewUserMessage(anthropic.NewTextBlock("there")),
	}

	before := len(readEventLines(t))
	_, _, err := r.RunOneStep(context.Background(), provider.DefaultModel, conv)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	lines := readEventLines(t)
	if got := len(lines) - before; got != 1 {
		t.Fatalf("expected exactly one new event line, got %d (before=%d after=%d)", got, before, len(lines))
	}

	// Validate last event fields
	var m map[string]any
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &m); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if m["event"] != "window_prepared" {
		t.Errorf("want event=window_prepared, got %v", m["event"])
	}
	if m["model"] != string(provider.DefaultModel) {
		t.Errorf("unexpected model: %v", m["model"])
	}
	if _, ok := m["turn_id"].(string); !ok || m["turn_id"].(string) == "" {
		t.Errorf("turn_id missing or empty")
	}
	if v, ok := m["over_budget_newest"].(bool); !ok || v {
		t.Errorf("expected over_budget_newest=false, got %v", m["over_budget_newest"])
	}
}

func TestRunner_VerboseSummary_OnOff(t *testing.T) {
	t.Setenv("AGT_TOKEN_BUDGET", "10")
	_ = chdirTemp(t)

	capReq := &capture{}
	fake := &fakeTransport{respStatus: 200, respBody: []byte(`{"content":[],"role":"assistant"}`), captured: capReq}
	cli := newClientWithTransport(fake)
	r := runner.New(cli, tools.Registry())
	conv := []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock("hello"))}

	// On
	t.Setenv("AGT_VERBOSE_WINDOW_LOGS", "1")
	out := captureStdout(t, func() {
		_, _, err := r.RunOneStep(context.Background(), provider.DefaultModel, conv)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
	})
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 verbose line, got %d: %q", len(lines), out)
	}
	if !strings.Contains(lines[0], "window: model=") {
		t.Errorf("missing window summary prefix: %q", lines[0])
	}

	// Off
	t.Setenv("AGT_VERBOSE_WINDOW_LOGS", "0")
	out2 := captureStdout(t, func() {
		_, _, err := r.RunOneStep(context.Background(), provider.DefaultModel, conv)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
	})
	if strings.TrimSpace(out2) != "" {
		t.Fatalf("expected no verbose output, got %q", out2)
	}
}

func TestRunner_EmissionBeforeFastFail_And_VerboseLine(t *testing.T) {
	t.Setenv("AGT_TOKEN_BUDGET", "1")
	t.Setenv("AGT_OBSERVE_JSON", "1")
	_ = chdirTemp(t)

	capReq := &capture{}
	fake := &fakeTransport{respStatus: 200, respBody: []byte(`{"content":[],"role":"assistant"}`), captured: capReq}
	cli := newClientWithTransport(fake)
	r := runner.New(cli, tools.Registry())
	conv := []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock("hello"))}

	// Without verbose: must emit event and make no HTTP call
	before := len(readEventLines(t))
	_, _, err := r.RunOneStep(context.Background(), provider.DefaultModel, conv)
	if err == nil || !strings.Contains(err.Error(), "newest group exceeds AGT_TOKEN_BUDGET") {
		t.Fatalf("expected over-budget newest error, got %v", err)
	}
	if capReq.body != nil {
		t.Fatalf("expected no HTTP call on fast-fail; got body len=%d", len(capReq.body))
	}

	lines := readEventLines(t)
	if len(lines) == before {
		t.Fatalf("expected an emitted event line before error; none found")
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &m); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if v, ok := m["over_budget_newest"].(bool); !ok || !v {
		t.Errorf("expected over_budget_newest=true, got %v", m["over_budget_newest"])
	}

	// Variant: verbose-on-fast-fail prints exactly one line
	t.Setenv("AGT_VERBOSE_WINDOW_LOGS", "1")
	out := captureStdout(t, func() {
		_, _, _ = r.RunOneStep(context.Background(), provider.DefaultModel, conv)
	})
	vlines := strings.Split(strings.TrimSpace(out), "\n")
	if len(vlines) != 1 {
		t.Fatalf("expected 1 verbose line on fast-fail, got %d: %q", len(vlines), out)
	}
}

func TestRunner_TurnID_Propagation(t *testing.T) {
	t.Setenv("AGT_TOKEN_BUDGET", "10")
	t.Setenv("AGT_OBSERVE_JSON", "1")
	_ = chdirTemp(t)

	fake := &fakeTransport{respStatus: 200, respBody: []byte(`{"content":[],"role":"assistant"}`), captured: &capture{}}
	cli := newClientWithTransport(fake)
	r := runner.New(cli, tools.Registry())
	conv := []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock("ping"))}

	// Case 1: explicit turn ID
	ctx := telemetry.WithTurnID(context.Background(), "turn-abc")
	_, _, err := r.RunOneStep(ctx, provider.DefaultModel, conv)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	lines := readEventLines(t)
	if len(lines) == 0 {
		t.Fatal("no events written")
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &m); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if m["turn_id"] != "turn-abc" {
		t.Errorf("expected turn_id=turn-abc, got %v", m["turn_id"])
	}

	// Case 2: generated turn ID (non-empty)
	before := len(lines)
	_, _, err = r.RunOneStep(context.Background(), provider.DefaultModel, conv)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	lines2 := readEventLines(t)
	if len(lines2) <= before {
		t.Fatal("expected another event line")
	}
	var m2 map[string]any
	if err := json.Unmarshal([]byte(lines2[len(lines2)-1]), &m2); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if s, ok := m2["turn_id"].(string); !ok || strings.TrimSpace(s) == "" {
		t.Errorf("expected non-empty generated turn_id, got %v", m2["turn_id"])
	}
}

func TestRunner_NoEmit_NoVerbose_WhenFlagsOff(t *testing.T) {
	t.Setenv("AGT_TOKEN_BUDGET", "10")
	_ = chdirTemp(t)

	fake := &fakeTransport{respStatus: 200, respBody: []byte(`{"content":[],"role":"assistant"}`), captured: &capture{}}
	cli := newClientWithTransport(fake)
	r := runner.New(cli, tools.Registry())
	conv := []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock("ok"))}

	out := captureStdout(t, func() {
		_, _, err := r.RunOneStep(context.Background(), provider.DefaultModel, conv)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
	})
	if strings.TrimSpace(out) != "" {
		t.Fatalf("expected no verbose output, got %q", out)
	}

	if _, err := os.Stat(".agent"); !os.IsNotExist(err) {
		t.Fatalf("expected no .agent directory when AGT_OBSERVE_JSON is off")
	}
}
