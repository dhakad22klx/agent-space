package agent

import (
	"context"
	"fmt"
	"sync"

	providers "agent-harness/providers"
	tools "agent-harness/tools"

	"github.com/joho/godotenv"
)

// systemPrompt frames the job for the model and nudges it to reach for tools
// instead of guessing.
const systemPrompt = `You are agent-space, an assistant running in the user's terminal.
You have tools that inspect the user's machine. When answering needs the real
contents of a file, call read_file instead of guessing, and base the answer only
on what you read. For anything else the shell can tell you or do, call run_bash
and answer from what it printed.

A tool that fails still tells you something. When read_file reports that a path
does not exist but lists files with similar names, pick the one the user most
likely meant and call read_file again with that exact path; only ask the user
when the list is empty or genuinely ambiguous. Stop calling tools once you can
answer.

read_file is not confined to the working directory: it reads any path on this
machine, and on a miss it searches for similar names. Never claim you can only
see the current project, and never conclude a file is absent from anything but
what a tool actually reported. When the question is only whether a file exists
or where it lives, answer from the search result itself; do not read the file,
which may hold private data the user did not ask to see.

run_bash runs whatever command you give it, so keep commands to what the user
asked for. Prefer the narrow command over the sweeping one, and when a command
would delete, overwrite or send data anywhere, describe it and let the user say
yes before running it. A non-zero exit is information, not a dead end: read the
output it carries before deciding what to do next.`

// defaultMaxSteps bounds a single Run so a confused model cannot call tools
// forever.
const defaultMaxSteps = 10

// Agent runs the ask-model / run-tools / ask-again loop and keeps the
// conversation history across turns.
// One agent serves the whole process (see GetAgent), and the Telegram poll calls
// Run from its own goroutine. mu guards the history for a whole turn, so a
// request from a phone waits for the local one instead of interleaving with it.
type Agent struct {
	provider providers.IProvider
	tools    *tools.Registry
	maxSteps int

	mu      sync.Mutex
	history []providers.Message

	// OnToolCall, when set, runs after each tool call so the UI can show what
	// the agent is doing.
	OnToolCall func(call providers.ToolCall, result providers.ToolResult)
}

// New builds an agent over a provider and the tools it is allowed to use.
func New(provider providers.IProvider, registry *tools.Registry) *Agent {
	return &Agent{
		provider: provider,
		tools:    registry,
		maxSteps: defaultMaxSteps,
	}
}

// The one agent this process runs. Two things reach for it — the prompt and the
// Telegram link — and one instance means one conversation, wherever the request
// came from. sync.Once because only the building has to happen once; every later
// call reads a pointer the Once already published safely.
var (
	agentInstance *Agent
	agentOnce     sync.Once
)

// GetAgent returns that agent, building it on the first call.
//
// The first caller's provider is the one it keeps; a later call cannot replace
// it. Tools are chosen here rather than by the caller, so adding one is a change
// to this function alone.
//
// A nil provider means no model is configured, and the answer is a nil agent
// rather than one that would panic when asked. The Once is left unspent, so a
// later call with a working provider still builds; callers read nil as
// "unavailable".
func GetAgent(provider providers.IProvider) *Agent {
	if provider == nil {
		return nil
	}

	agentOnce.Do(func() {
		agentInstance = New(provider, tools.NewRegistry(tools.NewRunBash()))
	})

	return agentInstance
}

// Run answers one user prompt, running as many tool rounds as the model asks
// for along the way.
func (a *Agent) Run(ctx context.Context, prompt string) (string, error) {
	env, err := godotenv.Read(".env")
	if err != nil {
		return "", fmt.Errorf("read .env: %w", err)
	}
	mockAgentCall := env["MOCK_AGENT_CALL"]
	if mockAgentCall == "true" {
		return "Agent call is mocked, to turn it on type `/on` and to turn it back off type `/off`.", nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()

	a.history = append(a.history, providers.Message{Role: providers.RoleUser, Text: prompt})

	for step := 0; step < a.maxSteps; step++ {
		reply, err := a.provider.Chat(ctx, systemPrompt, a.history, a.tools.Schemas())
		if err != nil {
			return "", err
		}

		// Everything the model says stays in the history, so the next request
		// carries the full trail of calls and results.
		a.history = append(a.history, reply)

		// No tool calls means the model is done thinking: this is the answer.
		if len(reply.ToolCalls) == 0 {
			return reply.Text, nil
		}

		results := make([]providers.ToolResult, 0, len(reply.ToolCalls))
		for _, call := range reply.ToolCalls {
			result := a.runTool(ctx, call)
			if a.OnToolCall != nil {
				a.OnToolCall(call, result)
			}
			results = append(results, result)
		}

		// Hand every result back and let the model take the next step.
		a.history = append(a.history, providers.Message{Role: providers.RoleTool, ToolResults: results})
	}

	return "", fmt.Errorf("stopped after %d tool rounds without a final answer", a.maxSteps)
}

// Reset forgets the conversation so far. One agent answers everywhere, so
// `reset` at the prompt also drops what was said over Telegram.
func (a *Agent) Reset() {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.history = nil
}

// runTool executes one call. Failures come back as tool output rather than as a
// Go error, so the model can read what went wrong and try something else.
func (a *Agent) runTool(ctx context.Context, call providers.ToolCall) providers.ToolResult {
	result := providers.ToolResult{ID: call.ID, Name: call.Name}

	tool, ok := a.tools.Get(call.Name)
	if !ok {
		result.Output = fmt.Sprintf("unknown tool %q", call.Name)
		result.IsError = true
		return result
	}

	output, err := tool.Call(ctx, call.Args)
	if err != nil {
		result.Output = err.Error()
		result.IsError = true
		return result
	}

	result.Output = output

	return result
}
