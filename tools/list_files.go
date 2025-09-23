package tools

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/petasbytes/go-agent/internal/fsops"
)

type ListFilesInput struct {
	Path     string `json:"path,omitempty" jsonschema_description:"Optional relative path to list files from (defaults to current directory)."`
	Page     int    `json:"page,omitempty" jsonschema_description:"1-based page number (default 1)."`
	PageSize int    `json:"page_size,omitempty" jsonschema_description:"Page size (default 200)."`
}

// defaultListFilesPageSize is the fallback page size when page_size <= 0.
const defaultListFilesPageSize = 200

var ListFilesDefinition = ToolDefinition{
	Name:        "list_files",
	Description: "List names of files in a directory within the workspace (non-recursive).",
	InputSchema: ListFilesInputSchema,
	Function:    ListFiles,
}

var ListFilesInputSchema = GenerateSchema[ListFilesInput]()

// ListFiles lists non-recursive directory entries under the sandbox via fsops,
// then applies deterministic sorting and simple paging at the tool layer.
// Defaults:
//   - page: 1 when <= 0
//   - page_size: 200 when <= 0
//
// Contract: returns a JSON-encoded []string to preserve existing tool behaviour.
func ListFiles(input json.RawMessage) (string, error) {
	var in ListFilesInput
	if err := json.Unmarshal(input, &in); err != nil {
		return "", err
	}
	page := in.Page
	// Default benign inputs for LLM callers to keep behaviour predicable.
	if page <= 0 {
		page = 1
	}
	pageSize := in.PageSize
	if pageSize <= 0 {
		pageSize = defaultListFilesPageSize
	}

	namesJSON, err := fsops.ListFiles(in.Path)
	if err != nil {
		return "", err
	}
	var names []string
	if err := json.Unmarshal([]byte(namesJSON), &names); err != nil {
		return "", fmt.Errorf("invalid list_files payload: %w", err)
	}
	// Standardise order so paging is deterministic across filesystems.
	sort.Strings(names)

	start := (page - 1) * pageSize
	// Out-of-range page returns an empty JSON array; keep the output contract.
	if start >= len(names) {
		return "[]", nil
	}
	end := start + pageSize
	if end > len(names) {
		end = len(names)
	}
	paged := names[start:end]

	b, err := json.Marshal(paged)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
