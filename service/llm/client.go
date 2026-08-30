package llm

import "context"

// Request is provider-neutral. Provider-specific options stay inside each
// Client implementation.
type Request struct {
	System    string
	Prompt    string
	MaxTokens int
}

type Client interface {
	Complete(ctx context.Context, request Request) (string, error)
}
