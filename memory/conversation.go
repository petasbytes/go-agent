package memory

import (
	"encoding/json"
	"errors"
	"os"
)

// Message is a minimal persisted view of a chat turn.
// For simplicity, currently storing only text. Tool blocks are transient.
type Message struct {
	Role string `json:"role"`
	Text string `json:"text,omitempty"`
}

func LoadConversation(path string) ([]Message, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var msgs []Message
	if err := json.Unmarshal(b, &msgs); err != nil {
		return nil, err
	}
	return msgs, nil
}

func SaveConversation(path string, msgs []Message) error {
	b, err := json.MarshalIndent(msgs, "", " ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}
