package provider

import (
	"github.com/anthropics/anthropic-sdk-go"
)

// NewAnthropicClient returns a client using API key from the env.
func NewAnthropicClient() *anthropic.Client {
	c := anthropic.NewClient()
	return &c
}

// DefaultModel centralises the default Anthropic model for the project
const DefaultModel = anthropic.ModelClaude3_7SonnetLatest
