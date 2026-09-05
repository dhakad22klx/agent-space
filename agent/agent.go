package agent

import (
	"context"
	"fmt"
	"sync"

	humanintheloop "agent-harness/agent/human-in-the-loop"
	state "agent-harness/agent/state"
	providers "agent-harness/providers"
	tools "agent-harness/tools"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
)

// systemPrompt frames the job for the model. It says nothing about individual
// tools: the catalogue goes with every request and each tool's schema describes
// itself, so a second copy here would only be the one that goes stale. What
// stays is what no single schema can say - how to treat what tools report, and
// when not to call one at all.
const systemPrompt = `You are agent-space, an assistant running in the user's terminal.

Answer from what your tools actually report, never from what you assume. Each
tool describes when it applies and what it cannot do; read those descriptions
and pick by what the question needs.

Gather with one tool, then write in your own words. Output another tool handed
you is material for your answer, not the answer itself, and not something to
paste into a tool that sends text somewhere.

A call may be held for the user to approve before it runs. If a result says the
call was rejected or changed, that is the user's decision: say so plainly and do
not reach for another tool to get the same thing done.

Most questions need no tool at all. Explaining, summarising, or answering from
what a tool already told you is your own work, so do not call a tool to confirm
something you have been told, and stop calling tools once you can answer.`

// defaultMaxSteps bounds a single Run so a confused model cannot call tools
// forever.
const defaultMaxSteps = 10

// pausedForApproval is what Run answers once a call has been handed to a human.
const pausedForApproval = "Processed for Human approval"

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
	session string // names this run in the state store

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
		agentInstance = New(provider, tools.NewRegistry(
			tools.NewRunBash(),
			tools.NewSendUpdatesToManager(),
		))
	})

	return agentInstance
}

// Run answers one user prompt, running as many tool rounds as the model asks
// for along the way.
func (a *Agent) Run(ctx context.Context, prompt string, sessionID string) (string, error) {
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
			// The first call a human has to sign off on ends the turn. What ran
			// before it is kept, so an approval arriving later resumes instead
			// of running those tools a second time.
			if needHumanApproval(call.Name) {
				if err := a.pause(ctx, sessionID, call, results, step); err != nil {
					return "", err
				}

				return pausedForApproval, nil
			}

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

func needHumanApproval(toolName string) bool {
	return humanintheloop.RequiresApproval(toolName)
}

// pause saves the run to the state store and leaves it there. The results
// already gathered go into the history first, so the state holds everything up
// to the call that is waiting.
func (a *Agent) pause(ctx context.Context, sessionID string, call providers.ToolCall, done []providers.ToolResult, step int) error {
	if len(done) > 0 {
		a.history = append(a.history, providers.Message{Role: providers.RoleTool, ToolResults: done})
	}

	store, err := state.OpenRedis(ctx)
	if err != nil {
		return err
	}
	defer store.Close()

	return store.Put(ctx, sessionID, state.AgentState{
		SessionID: sessionID,
		History:   a.history,
		PendingApproval: &state.PendingApproval{
			ID:             uuid.NewString(),
			ToolCall:       call,
			ApprovalStatus: "PENDING",
		},
		Status: state.StatusWaitingApproval,
		Step:   step,
	})
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
