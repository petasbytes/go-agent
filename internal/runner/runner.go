package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/petasbytes/go-agent/internal/telemetry"
	"github.com/petasbytes/go-agent/internal/windowing"
	"github.com/petasbytes/go-agent/tools"
)

type Runner struct {
	Client *anthropic.Client
	Tools  []tools.ToolDefinition
}

func New(client *anthropic.Client, toolDefs []tools.ToolDefinition) *Runner {
	return &Runner{Client: client, Tools: toolDefs}
}

func (r *Runner) anthropicTools() []anthropic.ToolUnionParam {
	out := make([]anthropic.ToolUnionParam, 0, len(r.Tools))
	for _, t := range r.Tools {
		out = append(out, anthropic.ToolUnionParam{OfTool: &anthropic.ToolParam{
			Name:        t.Name,
			Description: anthropic.String(t.Description),
			InputSchema: t.InputSchema,
		}})
	}
	return out
}

// RunOneStep sends the conversation and either prints text or returns tool results to be appended.
func (r *Runner) RunOneStep(ctx context.Context, model anthropic.Model, conv []anthropic.MessageParam) (*anthropic.Message, []anthropic.ContentBlockParamUnion, error) {
	v := os.Getenv("AGT_TOKEN_BUDGET")
	if v == "" {
		return nil, nil, fmt.Errorf("AGT_TOKEN_BUDGET not set; export it then try again")
	}
	budget, err := strconv.Atoi(v)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid AGT_TOKEN_BUDGET %q: %w", v, err)
	}

	// Prepare pair-safe, budgeted window
	counter := windowing.HeuristicCounter{}
	window, stats := windowing.PrepareSendWindow(conv, budget, counter)

	// Get turnID from context if present, else generate once for this call.
	turnID, ok := telemetry.TurnIDFromContext(ctx)
	if !ok {
		turnID = fmt.Sprintf("turn-%d", time.Now().UnixNano())
	}

	ctx = telemetry.WithTurnID(ctx, turnID)

	telemetry.Emit("window_prepared", map[string]any{
		"turn_id":            turnID,
		"model":              string(model),
		"budget":             stats.Budget,
		"total_estimated":    stats.Total,
		"included_groups":    stats.IncludedGroups,
		"skipped_groups":     stats.SkippedGroups,
		"over_budget_newest": stats.OverBudgetNewest,
	})

	if os.Getenv("AGT_VERBOSE_WINDOW_LOGS") == "1" {
		fmt.Printf(
			"window: model=%s budget=%d est_total=%d groups_in=%d groups_skip=%d newest_over=%t\n",
			string(model), stats.Budget, stats.Total, stats.IncludedGroups, stats.SkippedGroups, stats.OverBudgetNewest,
		)
	}

	// With tool caps the newest group should always fit within AGT_TOKEN_BUDGET.
	// If not, treat it as a misconfiguration (e.g. too-low budget or caps not applied) and
	// fail fast with error.
	if stats.OverBudgetNewest {
		return nil, nil, fmt.Errorf("windowing: newest group exceeds AGT_TOKEN_BUDGET; increase budget with headroom or tighten tool caps")
	}

	// Build final request params from the prepared window; gate Tools on calibration mode
	params := anthropic.MessageNewParams{
		Model:     model,
		MaxTokens: int64(1024),
		Messages:  window,
	}
	// Only include tools when NOT in calibration mode
	if !telemetry.CalibrationModeEnabled() {
		params.Tools = r.anthropicTools()
	}

	msg, err := r.Client.Messages.New(ctx, params)
	if err != nil {
		return nil, nil, err
	}
	toolResults := []anthropic.ContentBlockParamUnion{}
	for _, block := range msg.Content {
		switch v := block.AsAny().(type) {
		case anthropic.TextBlock:
			fmt.Printf("\u001b[93mClaude\u001b[0m: %s\n", v.Text)
		case anthropic.ToolUseBlock:
			// Pass raw JSON input through to the tool implementation
			input := json.RawMessage(v.JSON.Input.Raw())
			res := r.execTool(ctx, v.ID, v.Name, input)
			toolResults = append(toolResults, res)
		}
	}
	return msg, toolResults, nil
}

func (r *Runner) execTool(ctx context.Context, id, name string, input json.RawMessage) anthropic.ContentBlockParamUnion {
	var def *tools.ToolDefinition
	for i := range r.Tools {
		if r.Tools[i].Name == name {
			def = &r.Tools[i]
			break
		}
	}

	turnID, _ := telemetry.TurnIDFromContext(ctx)

	// Helper to emit a tool_exec event
	emit := func(durationMs int64, inputSize int, outputSize int, errStr string) {
		fields := map[string]any{
			"tool_name":   name,
			"duration_ms": durationMs,
			"input_size":  inputSize,
			"output_size": outputSize,
			"turn_id":     turnID,
		}
		if errStr != "" {
			fields["error"] = errStr
		} else {
			fields["error"] = nil
		}
		telemetry.Emit("tool_exec", fields)
	}

	start := time.Now()
	inSize := len(input)

	// Handle "tool not found" as an error result and emit telemetry
	if def == nil {
		emit(time.Since(start).Milliseconds(), inSize, 0, "tool not found")
		return anthropic.NewToolResultBlock(id, "tool not found", true)
	}

	// Execute the tool
	resp, err := def.Function(input)
	if err != nil {
		// Emit a generic error string to avoid leaking raw payloads in telemetry
		emit(time.Since(start).Milliseconds(), inSize, 0, "tool error")
		// Preserve detailed error message in the tool result content returned to the model
		return anthropic.NewToolResultBlock(id, err.Error(), true)
	}
	outSize := len(resp)
	emit(time.Since(start).Milliseconds(), inSize, outSize, "")
	return anthropic.NewToolResultBlock(id, resp, false)
}
