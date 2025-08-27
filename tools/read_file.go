package tools

import (
	"encoding/json"
	"os"
)

type ReadFileInput struct {
	Path string `json:"path" jsonschema_description:"The relative path of a file in the working directory."`
}

var ReadFileDefinition = ToolDefinition{
	Name:        "read_file",
	Description: "Read contents of a relative file path. Do not use this tool with directory names.",
	InputSchema: ReadFileInputSchema,
	Function:    ReadFile,
}

var ReadFileInputSchema = GenerateSchema[ReadFileInput]()

func ReadFile(input json.RawMessage) (string, error) {
	var in ReadFileInput
	if err := json.Unmarshal(input, &in); err != nil {
		return "", err
	}
	b, err := os.ReadFile(in.Path)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
