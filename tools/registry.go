package tools

// Registry returns all tool definitions wired for the agent
func Registry() []ToolDefinition {
	return []ToolDefinition{ReadFileDefinition, ListFilesDefinition, EditFileDefinition}
}
