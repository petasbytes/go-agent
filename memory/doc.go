// Package memory provides minimal conversation persistence.
//
// Persistence model:
//   - Only text messages are stored (role + text). Tool blocks are transient.
//   - Keeping initial storage simple for now; to be extended.
package memory
