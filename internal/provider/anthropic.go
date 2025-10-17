package provider

import (
	"github.com/anthropics/anthropic-sdk-go"
)

// NewAnthropicClient returns a client using API key from the env.
func NewAnthropicClient() *anthropic.Client {
	c := anthropic.NewClient()
	return &c
}

const DefaultModel = anthropic.ModelClaude3_7SonnetLatest
const APIVersion = "2023-06-01"
