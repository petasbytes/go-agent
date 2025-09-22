package tools

import (
	"encoding/json"
	"strings"

	"github.com/petasbytes/go-agent/internal/fsops"
)

type ReadFileInput struct {
	Path   string `json:"path" jsonschema_description:"Relative file path."`
	Offset int    `json:"offset,omitempty" jsonschema_description:"Line offset (0-based) to start reading from."`
	Limit  int    `json:"limit,omitempty" jsonschema_description:"Maximum lines to return from offset (default 200)."`
}

// defaultReadFileLimit is the fallback page size when limit <= 0.
const defaultReadFileLimit = 200

// truncationSentinel marks that more content is available via offset/limit.
const truncationSentinel = "-- truncated; use offset/limit to fetch more --\n"

var ReadFileDefinition = ToolDefinition{
	Name:        "read_file",
	Description: "Read the contents of a file addressed by a relative file path within the workspace. Directory paths and unsafe paths are rejected.",
	InputSchema: ReadFileInputSchema,
	Function:    ReadFile,
}

var ReadFileInputSchema = GenerateSchema[ReadFileInput]()

// ReadFile reads a file via fsops (sandbox/policy) and applies small, deterministic
// caps for LLM-facing pagination:
//   - offset: 0-based starting line (negatives clamped to 0)
//   - limit: number of lines to return (<= defaults to 200)
//
// If not all lines are returned, it appends a trailing sentinel to signal pagination.
// Rational: keep tool results predictably small for windowing/token heuristics.
func ReadFile(input json.RawMessage) (string, error) {
	var in ReadFileInput
	if err := json.Unmarshal(input, &in); err != nil {
		return "", err
	}

	content, err := fsops.ReadFile(in.Path)
	if err != nil {
		return "", err
	}

	limit := in.Limit
	// Be forgiving for LLM callers: clamp negatives/defaults instead of failing.
	// Safety/policy violations still surface via fsops/safety as ToolError.
	if limit <= 0 {
		limit = defaultReadFileLimit
	}
	offset := in.Offset
	if offset < 0 {
		offset = 0
	}

	lines := strings.Split(content, "\n")
	if offset > len(lines) {
		offset = len(lines)
	}
	end := offset + limit
	if end > len(lines) {
		end = len(lines)
	}

	// If we didn't include all lines, mark truncation with a sentinel.
	// Ensure the chunk ends with a newline so the sentinel is on its own line.
	truncated := end < len(lines)
	out := strings.Join(lines[offset:end], "\n")
	if truncated {
		if !strings.HasSuffix(out, "\n") {
			out += "\n"
		}
		out += truncationSentinel
	}
	return out, nil
}
