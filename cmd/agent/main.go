package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/petasbytes/go-agent/internal/provider"
	"github.com/petasbytes/go-agent/internal/runner"
	"github.com/petasbytes/go-agent/memory"
	"github.com/petasbytes/go-agent/tools"
)

func main() {
	// Basic env check (SDK also reads API key)
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		fmt.Println("Missing ANTHROPIC_API_KEY; export it before running.")
		os.Exit(1)
	}

	// Load prior conversation if exists
	persistPath := "conversation.json"
	persisted, err := memory.LoadConversation(persistPath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(os.Stderr, "warning: failed to load persisted conversation: %v\n", err)
		}
	}

	client := provider.NewAnthropicClient()
	r := runner.New(client, tools.Registry())
	model := provider.DefaultModel

	// Build SDK conversation from persisted messages text
	conv := make([]anthropic.MessageParam, 0, len(persisted))
	for _, m := range persisted {
		if m.Role == "user" {
			conv = append(conv, anthropic.NewUserMessage(anthropic.NewTextBlock(m.Text)))
		} else {
			conv = append(conv, anthropic.NewAssistantMessage(anthropic.NewTextBlock(m.Text)))
		}
	}

	// Set up graceful shutdown on Ctrl-C (SIGINT) / SIGTERM
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sigch := make(chan os.Signal, 1)
	signal.Notify(sigch, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigch)
	go func() {
		<-sigch
		fmt.Println("\nExiting...")
		cancel()
	}()

	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("Chat with Claude (Ctrl-C to quit)")

	// stdin reader goroutine -> lines into channel
	inputCh := make(chan string)
	go func() {
		for scanner.Scan() {
			inputCh <- scanner.Text()
		}
		close(inputCh)
	}()

outer:
	for {
		fmt.Print("\u001b[94mYou\u001b[0m: ")
		var (
			user string
			ok   bool
		)
		select {
		case <-ctx.Done():
			break outer
		case user, ok = <-inputCh:
			if !ok {
				break outer
			}
		}
		conv = append(conv, anthropic.NewUserMessage(anthropic.NewTextBlock(user)))

		// Track assistant visible text to persist after the turn
		var lastAssistantText string
		for {
			// No windowing for now; send full conversation buffer
			msg, toolResults, err := r.RunOneStep(ctx, model, conv)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				break
			}
			conv = append(conv, msg.ToParam())
			// Collect assistant text blocks from this message
			for _, b := range msg.Content {
				if tb, ok := b.AsAny().(anthropic.TextBlock); ok {
					if tb.Text != "" {
						if lastAssistantText == "" {
							lastAssistantText = tb.Text
						} else {
							lastAssistantText += "\n" + tb.Text
						}
					}
				}
			}
			if len(toolResults) == 0 {
				break // done with assistant turn
			}
			// Provide tool results as a user message back to the model
			conv = append(conv, anthropic.NewUserMessage(toolResults...))
		}

		// Persist minimal text-only transcript (user + assistant)
		persisted = append(persisted, memory.Message{Role: "user", Text: user})
		if strings.TrimSpace(lastAssistantText) != "" {
			persisted = append(persisted, memory.Message{Role: "assistant", Text: lastAssistantText})
		}
		// Keeping tool blocks transient for now (simpler persistence)
		if err := memory.SaveConversation(persistPath, persisted); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to save conversation: %v\n", err)
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: stdin read error: %v\n", err)
	}
}
