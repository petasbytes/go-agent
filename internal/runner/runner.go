package runner

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
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
	msg, err := r.Client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     model,
		MaxTokens: int64(1024),
		Messages:  conv,
		Tools:     r.anthropicTools(),
	})
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
			res := r.execTool(v.ID, v.Name, input)
			toolResults = append(toolResults, res)
		}
	}
	return msg, toolResults, nil
}

func (r *Runner) execTool(id, name string, input json.RawMessage) anthropic.ContentBlockParamUnion {
	var def *tools.ToolDefinition
	for i := range r.Tools {
		if r.Tools[i].Name == name {
			def = &r.Tools[i]
			break
		}
	}
	if def == nil {
		return anthropic.NewToolResultBlock(id, "tool not found", true)
	}
	resp, err := def.Function(input)
	if err != nil {
		return anthropic.NewToolResultBlock(id, err.Error(), true)
	}
	return anthropic.NewToolResultBlock(id, resp, false)
}
