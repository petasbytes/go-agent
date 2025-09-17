package tools

import (
	"encoding/json"

	"github.com/petasbytes/go-agent/internal/fsops"
)

type ReadFileInput struct {
	Path string `json:"path" jsonschema_description:"Relative file path."`
}

var ReadFileDefinition = ToolDefinition{
	Name:        "read_file",
	Description: "Read the contents of a file addressed by a relative file path within the workspace. Directory paths and unsafe paths are rejected.",
	InputSchema: ReadFileInputSchema,
	Function:    ReadFile,
}

var ReadFileInputSchema = GenerateSchema[ReadFileInput]()

func ReadFile(input json.RawMessage) (string, error) {
	var in ReadFileInput
	if err := json.Unmarshal(input, &in); err != nil {
		return "", err
	}
	return fsops.ReadFile(in.Path)
}
