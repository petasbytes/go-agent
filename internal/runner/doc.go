// Package runner coordinates message exchange with the Anthropic Messages API
// and dispatches tool calls.
//
// Invariant:
//   - tool_use and the corresponding tool_result are kept adjacent within a turn
//     to preserve execution context and simplify follow-up reasoning.
//
// Flow:
//
//	user(text) -> assistant(tool_use) -> user(tool_result) -> assistant(text)
package runner
