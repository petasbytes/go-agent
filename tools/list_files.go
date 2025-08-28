package tools

import (
	"encoding/json"
	"os"
)

type ListFilesInput struct {
	Path string `json:"path,omitempty" jsonschema_description:"Optional relative path to list files from. Defaults to current directory if not provided."`
}

var ListFilesDefinition = ToolDefinition{
	Name:        "list_files",
	Description: "List names of files in a directory (non-recursive).",
	InputSchema: ListFilesInputSchema,
	Function:    ListFiles,
}

var ListFilesInputSchema = GenerateSchema[ListFilesInput]()

func ListFiles(input json.RawMessage) (string, error) {
	var in ListFilesInput
	if err := json.Unmarshal(input, &in); err != nil {
		return "", err
	}

	dir := "."
	if in.Path != "" {
		dir = in.Path
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}

	files := make([]string, 0, len(entries))
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() {
			name += "/"
		}
		files = append(files, name)
	}

	b, err := json.Marshal(files)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
