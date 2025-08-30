// Package tools defines tool contracts and implementations.
//
// Includes:
//   - ToolDefinition: name, description, JSON input schema, handler.
//   - GenerateSchema[T](): derive JSON Schema from Go structs.
//   - File tools: read_file, list_files (non-recursive), edit_file.
//   - Invariants: tool_use and its corresponding tool_result remain adjacent within a turn
package tools
