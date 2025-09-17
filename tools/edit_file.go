package tools

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/petasbytes/go-agent/internal/fsops"
)

type EditFileInput struct {
	Path   string `json:"path" jsonschema_description:"Target relative file path"`
	OldStr string `json:"old_str" jsonschema_description:"Exact text to replace; must be present once when editing an existing file."`
	NewStr string `json:"new_str" jsonschema_description:"New text to write or replace old_str with"`
}

var EditFileDefinition = ToolDefinition{
	Name: "edit_file",
	Description: `Create or modify a text file addressed by a relative path within the workspace.

When old_str is empty and the file doesn’t exist, a new file is created.

When editing an existing file, all occurrences of old_str are replaced with new_str; old_str and new_str must be different.
`,
	InputSchema: EditFileInputSchema,
	Function:    EditFile,
}

var EditFileInputSchema = GenerateSchema[EditFileInput]()

func EditFile(input json.RawMessage) (string, error) {
	editFileInput := EditFileInput{}
	err := json.Unmarshal(input, &editFileInput)
	if err != nil {
		return "", err
	}

	if editFileInput.Path == "" || editFileInput.OldStr == editFileInput.NewStr {
		return "", fmt.Errorf("invalid edit parameters")
	}

	// Try to read the existing file via fsops.
	oldContent, readErr := fsops.ReadFile(editFileInput.Path)
	if readErr != nil {
		// If file does not exist and OldStr is empty, create new file with NewStr
		if editFileInput.OldStr == "" {
			if err := fsops.WriteFile(editFileInput.Path, editFileInput.NewStr); err != nil {
				return "", err
			}
			return fmt.Sprintf("Successfully created file %s", editFileInput.Path), nil
		}
		// Otherwise propagate the read error (could be ToolError or other I/O error)
		return "", readErr
	}

	// If the file exists, require a non-empty old_str to avoid ambiguous behaviour
	if editFileInput.OldStr == "" {
		return "", fmt.Errorf("old_str must be provided when editing an existing file")
	}

	// Replace existing content
	newContent := strings.Replace(oldContent, editFileInput.OldStr, editFileInput.NewStr, -1)
	if newContent == oldContent && editFileInput.OldStr != "" {
		return "", fmt.Errorf("old_str not found in file")
	}

	if err := fsops.WriteFile(editFileInput.Path, newContent); err != nil {
		return "", err
	}
	return "OK", nil
}
