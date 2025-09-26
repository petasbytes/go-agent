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

const defaultReadFileLimit = 200 // fallback page size when limit <= 0
const truncationSentinel = "-- truncated; use offset/limit to fetch more --\n"
const maxLineRunes = 2000     // per-line clamp
const overallRuneCap = 12_000 // overall cap after join

var ReadFileDefinition = ToolDefinition{
	Name:        "read_file",
	Description: "Read the contents of a file addressed by a relative file path within the workspace. Directory paths and unsafe paths are rejected.",
	InputSchema: ReadFileInputSchema,
	Function:    ReadFile,
}

var ReadFileInputSchema = GenerateSchema[ReadFileInput]()

// Helper: clamp a string to at most n runes
func clampRunes(s string, n int) (string, bool) {
	if n <= 0 {
		return "", len([]rune(s)) > 0
	}
	r := []rune(s)
	if len(r) <= n {
		return s, false
	}
	return string(r[:n]), true
}

// ReadFile reads a file via fsops (sandbox/policy) and applies small, deterministic
// caps for LLM-facing pagination:
//   - offset: 0-based starting line (negatives clamped to 0)
//   - limit: number of lines to return (<= defaults to 200)
//
// If not all lines are returned, it appends a trailing sentinel to signal pagination.
// Rationale: keep tool results predictably small for windowing/token heuristics.
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
	if limit <= 0 {
		limit = defaultReadFileLimit // 200
	}
	offset := in.Offset
	if offset < 0 {
		offset = 0
	}

	// Split and select window
	lines := strings.Split(content, "\n")
	if offset > len(lines) {
		offset = len(lines)
	}
	end := offset + limit
	if end > len(lines) {
		end = len(lines)
	}

	// Clamp each line to maxLineRunes, tracking if any truncation occurred
	truncated := end < len(lines)
	for i := offset; i < end; i++ {
		if clamped, did := clampRunes(lines[i], maxLineRunes); did {
			lines[i] = clamped
			truncated = true
		}
	}

	out := strings.Join(lines[offset:end], "\n")

	// Apply overall cap after join
	if _, did := clampRunes(out, overallRuneCap); did {
		r := []rune(out)
		out = string(r[:overallRuneCap])
		truncated = true
	}

	// Ensure final newline and sentinel if any truncation took place
	if truncated {
		if !strings.HasSuffix(out, "\n") {
			out += "\n"
		}
		if !strings.HasSuffix(out, truncationSentinel) {
			out += truncationSentinel
		}
	}
	return out, nil
}
