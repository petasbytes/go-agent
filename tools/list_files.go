package tools

import (
	"encoding/json"

	"github.com/petasbytes/go-agent/internal/fsops"
)

type ListFilesInput struct {
	Path string `json:"path,omitempty" jsonschema_description:"Optional relative path to list files from (defaults to current directory)."`
}

var ListFilesDefinition = ToolDefinition{
	Name:        "list_files",
	Description: "List names of files in a directory within the workspace (non-recursive).",
	InputSchema: ListFilesInputSchema,
	Function:    ListFiles,
}

var ListFilesInputSchema = GenerateSchema[ListFilesInput]()

func ListFiles(input json.RawMessage) (string, error) {
	var in ListFilesInput
	if err := json.Unmarshal(input, &in); err != nil {
		return "", err
	}
	return fsops.ListFiles(in.Path)
}
