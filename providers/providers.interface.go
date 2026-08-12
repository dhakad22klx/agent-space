package providers

import (
	"context"

	tools "agent-harness/tools"
)

// IProvider is the contract every model provider implements, so the CLI and the
// agent depend on this rather than on one vendor's client.
type IProvider interface {
	// Model reports which model is being talked to.
	Model() string

	// Generate answers a single prompt: no history, no tools.
	Generate(ctx context.Context, input string) (string, error)

	// Chat sends the conversation and the tool catalogue, and returns the reply:
	// text, tool calls, or both.
	Chat(ctx context.Context, system string, history []Message, schemas []tools.Schema) (Message, error)
}

// Roles used in the conversation history.
const (
	RoleUser  = "user"  // what the person typed
	RoleModel = "model" // what the model answered
	RoleTool  = "tool"  // what the tools returned
)

// Message is one entry of history. A model message carries text, tool calls or
// both; a tool message carries one result per call.
type Message struct {
	Role        string
	Text        string
	ToolCalls   []ToolCall
	ToolResults []ToolResult

	// raw is the provider's own copy of a model turn, opaque to everyone else:
	// Gemini requires the thought signature attached to a function call to be
	// echoed back untouched, so a replayed turn uses this instead of the fields
	// above.
	raw any
}

// ToolCall is the model asking for one tool to run.
type ToolCall struct {
	ID   string
	Name string
	Args map[string]any
}

// ToolResult is what running a ToolCall produced.
type ToolResult struct {
	ID      string
	Name    string
	Output  string
	IsError bool
}
