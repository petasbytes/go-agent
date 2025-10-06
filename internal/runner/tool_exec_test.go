package runner_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/petasbytes/go-agent/internal/provider"
	"github.com/petasbytes/go-agent/internal/runner"
	"github.com/petasbytes/go-agent/internal/telemetry"
	"github.com/petasbytes/go-agent/tools"
)

func TestRunner_ToolExec_JSONL_Success(t *testing.T) {
	t.Setenv("AGT_TOKEN_BUDGET", "1000")
	t.Setenv("AGT_OBSERVE_JSON", "1")
	_ = chdirTemp(t)

	// Provider response triggers a tool_use with a small JSON input
	resp := `{
		"role": "assistant",
		"content": [
			{"type": "tool_use", "id": "t1", "name": "list_files", "input": {"path": "."}}
		]
	}`
	fake := &fakeTransport{respStatus: 200, respBody: []byte(resp), captured: &capture{}}
	cli := newClientWithTransport(fake)

	r := runner.New(cli, tools.Registry())
	conv := []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock("please list files"))}

	before := len(readEventLines(t))
	_, _, err := r.RunOneStep(context.Background(), provider.DefaultModel, conv)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	lines := readEventLines(t)
	if got := len(lines) - before; got < 2 { // window_prepared + tool_exec
		t.Fatalf("expected at least 2 new events, got %d", got)
	}

	// Find the last tool_exec event and validate fields
	var exec map[string]any
	for i := len(lines) - 1; i >= 0; i-- {
		var m map[string]any
		if err := json.Unmarshal([]byte(lines[i]), &m); err != nil {
			t.Fatalf("invalid JSON: %v", err)
		}
		if m["event"] == "tool_exec" {
			exec = m
			break
		}
	}
	if exec == nil {
		t.Fatal("no tool_exec event found")
	}

	if exec["tool_name"] != "list_files" {
		t.Errorf("tool_name: want list_files, got %v", exec["tool_name"])
	}
	if v, ok := exec["duration_ms"].(float64); !ok || v < 0 {
		t.Errorf("duration_ms should be >= 0, got %v", exec["duration_ms"])
	}
	// input size should be len({"path":"."}) without spaces
	if v, ok := exec["input_size"].(float64); !ok || v <= 0 {
		t.Errorf("input_size should be > 0, got %v", exec["input_size"])
	}
	if v, ok := exec["output_size"].(float64); !ok || v <= 0 {
		t.Errorf("output_size should be > 0, got %v", exec["output_size"])
	}
	if _, ok := exec["error"]; !ok {
		t.Errorf("missing error field")
	} else if exec["error"] != nil {
		t.Errorf("error should be null on success, got %v", exec["error"])
	}
	if s, ok := exec["turn_id"].(string); !ok || strings.TrimSpace(s) == "" {
		t.Errorf("turn_id missing or empty: %v", exec["turn_id"])
	}

	// Correlate with latest window_prepared turn_id
	var wp map[string]any
	for i := len(lines) - 1; i >= 0; i-- {
		var m map[string]any
		_ = json.Unmarshal([]byte(lines[i]), &m)
		if m["event"] == "window_prepared" {
			wp = m
			break
		}
	}
	if wp == nil {
		t.Fatal("no window_prepared event found")
	}
	if exec["turn_id"] != wp["turn_id"] {
		t.Errorf("turn_id mismatch between tool_exec and window_prepared: %v vs %v", exec["turn_id"], wp["turn_id"])
	}
}

func TestRunner_ToolExec_JSONL_HandlerError(t *testing.T) {
	t.Setenv("AGT_TOKEN_BUDGET", "1000")
	t.Setenv("AGT_OBSERVE_JSON", "1")
	_ = chdirTemp(t)

	// Tool that returns an error
	errTool := tools.ToolDefinition{
		Name:        "err_tool",
		Description: "always errors",
		InputSchema: tools.GenerateSchema[struct{}](),
		Function: func(input json.RawMessage) (string, error) {
			return "", fmt.Errorf("boom")
		},
	}

	// Provider asks to call err_tool with any input
	resp := `{
		"role": "assistant",
		"content": [
			{"type": "tool_use", "id": "e1", "name": "err_tool", "input": {"x": 1}}
		]
	}`
	fake := &fakeTransport{respStatus: 200, respBody: []byte(resp), captured: &capture{}}
	cli := newClientWithTransport(fake)
	r := runner.New(cli, []tools.ToolDefinition{errTool})
	conv := []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock("call err tool"))}

	before := len(readEventLines(t))
	_, _, err := r.RunOneStep(context.Background(), provider.DefaultModel, conv)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	lines := readEventLines(t)
	if len(lines) <= before {
		t.Fatal("expected new events written")
	}

	// Find tool_exec
	var exec map[string]any
	for i := len(lines) - 1; i >= 0; i-- {
		var m map[string]any
		_ = json.Unmarshal([]byte(lines[i]), &m)
		if m["event"] == "tool_exec" {
			exec = m
			break
		}
	}
	if exec == nil {
		t.Fatal("no tool_exec event found")
	}
	if exec["tool_name"] != "err_tool" {
		t.Errorf("tool_name: want err_tool, got %v", exec["tool_name"])
	}
	if exec["error"] == nil || exec["error"].(string) == "" {
		t.Errorf("expected non-empty error string, got %v", exec["error"])
	}
	if v, ok := exec["output_size"].(float64); !ok || v != 0 {
		t.Errorf("output_size should be 0 on error, got %v", exec["output_size"])
	}
}

func TestRunner_ToolExec_JSONL_ToolNotFound(t *testing.T) {
	t.Setenv("AGT_TOKEN_BUDGET", "1000")
	t.Setenv("AGT_OBSERVE_JSON", "1")
	_ = chdirTemp(t)

	// No matching tool in registry
	resp := `{
		"role": "assistant",
		"content": [
			{"type": "tool_use", "id": "nf1", "name": "does_not_exist", "input": {"a": 1}}
		]
	}`
	fake := &fakeTransport{respStatus: 200, respBody: []byte(resp), captured: &capture{}}
	cli := newClientWithTransport(fake)
	r := runner.New(cli, []tools.ToolDefinition{})
	conv := []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock("call missing"))}

	before := len(readEventLines(t))
	_, _, err := r.RunOneStep(context.Background(), provider.DefaultModel, conv)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	lines := readEventLines(t)
	if len(lines) <= before {
		t.Fatal("expected new events written")
	}

	var exec map[string]any
	for i := len(lines) - 1; i >= 0; i-- {
		var m map[string]any
		_ = json.Unmarshal([]byte(lines[i]), &m)
		if m["event"] == "tool_exec" {
			exec = m
			break
		}
	}
	if exec == nil {
		t.Fatal("no tool_exec event found")
	}
	if v, ok := exec["output_size"].(float64); !ok || v != 0 {
		t.Errorf("output_size should be 0 for not found, got %v", exec["output_size"])
	}
	if exec["error"] == nil || exec["error"].(string) == "" {
		t.Errorf("expected non-empty error string for not found, got %v", exec["error"])
	}
}

func TestRunner_ToolExec_Gating_Off_NoWrites(t *testing.T) {
	t.Setenv("AGT_TOKEN_BUDGET", "1000")
	// Do NOT set AGT_OBSERVE_JSON, keep it off
	_ = chdirTemp(t)

	resp := `{
		"role": "assistant",
		"content": [
			{"type": "tool_use", "id": "t1", "name": "list_files", "input": {"path": "."}}
		]
	}`
	fake := &fakeTransport{respStatus: 200, respBody: []byte(resp), captured: &capture{}}
	cli := newClientWithTransport(fake)
	r := runner.New(cli, tools.Registry())
	conv := []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock("please list files"))}

	_, _, err := r.RunOneStep(context.Background(), provider.DefaultModel, conv)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	// Assert .agent does not exist and no JSONL was written
	if _, err := os.Stat(".agent"); !os.IsNotExist(err) {
		t.Fatalf("expected no .agent directory when AGT_OBSERVE_JSON is off")
	}
}

func TestRunner_ToolExec_JSONL_TurnID_Propagation(t *testing.T) {
	t.Setenv("AGT_TOKEN_BUDGET", "1000")
	t.Setenv("AGT_OBSERVE_JSON", "1")
	_ = chdirTemp(t)

	resp := `{"role":"assistant","content":[{"type":"tool_use","id":"t1","name":"list_files","input":{"path":"."}}]}`
	fake := &fakeTransport{respStatus: 200, respBody: []byte(resp), captured: &capture{}}
	cli := newClientWithTransport(fake)
	r := runner.New(cli, tools.Registry())
	conv := []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock("please list files"))}

	ctx := telemetry.WithTurnID(context.Background(), "turn-xyz")
	_, _, err := r.RunOneStep(ctx, provider.DefaultModel, conv)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	var wp, exec map[string]any
	for _, line := range readEventLines(t) {
		var m map[string]any
		_ = json.Unmarshal([]byte(line), &m)
		switch m["event"] {
		case "window_prepared":
			wp = m
		case "tool_exec":
			exec = m
		}
	}
	if wp == nil || exec == nil {
		t.Fatal("missing window_prepared or tool_exec")
	}
	if wp["turn_id"] != "turn-xyz" {
		t.Errorf("window_prepared turn_id = %v", wp["turn_id"])
	}
	if exec["turn_id"] != "turn-xyz" {
		t.Errorf("tool_exec turn_id = %v", exec["turn_id"])
	}
}

func TestRunner_ToolExec_Privacy_NoRawPayloadLeak(t *testing.T) {
	t.Setenv("AGT_TOKEN_BUDGET", "1000")
	t.Setenv("AGT_OBSERVE_JSON", "1")
	_ = chdirTemp(t)

	secret := "__SECRET_NEVER_APPEAR__"
	// Input includes a distinctive secret string
	resp := fmt.Sprintf(`{
		"role": "assistant",
		"content": [
			{"type": "tool_use", "id": "t1", "name": "list_files", "input": {"path": %q}}
		]
	}`, secret)

	fake := &fakeTransport{respStatus: 200, respBody: []byte(resp), captured: &capture{}}
	cli := newClientWithTransport(fake)
	r := runner.New(cli, tools.Registry())
	conv := []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock("please list files"))}

	_, _, err := r.RunOneStep(context.Background(), provider.DefaultModel, conv)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	// Ensure no event line contains the raw secret string
	for _, line := range readEventLines(t) {
		if strings.Contains(line, secret) {
			t.Fatalf("raw payload leaked into telemetry: %q", line)
		}
	}
}
