package providers

import (
	"encoding/json"
	"fmt"
)

// A run that pauses for an approval is stored and picked up later by another
// process, which means the history has to survive JSON. The fields of Message
// do that on their own; the provider's own copy of a turn does not, because it
// is an unexported value of a type only that provider knows.
//
// So it travels as bytes plus the name of the provider that wrote it. Encoding
// needs nothing from the provider, since its turn is already a wire type of its
// API. Decoding does, so each provider registers how to read its own back.

// rawCodecs turns a stored turn back into the provider's own type. Providers
// register into it from init, before anything decodes, so it is only read
// afterwards and needs no locking.
var rawCodecs = map[string]func(json.RawMessage) (any, error){}

// registerRawCodec records how provider decodes its copy of a turn. Calling it
// twice for the same provider is a bug in that provider, not a runtime
// condition, so it panics rather than quietly taking the second one.
func registerRawCodec(provider string, decode func(json.RawMessage) (any, error)) {
	if _, exists := rawCodecs[provider]; exists {
		panic(fmt.Sprintf("providers: raw codec for %q registered twice", provider))
	}

	rawCodecs[provider] = decode
}

// withRaw attaches the provider's own copy of this turn.
func (m Message) withRaw(provider string, turn any) Message {
	m.rawProvider = provider
	m.rawTurn = turn

	return m
}

// rawOf returns the provider's own copy of this turn, or nil if the turn came
// from somewhere else. A provider asks for its own name and gets nothing for a
// history recorded against another model, which is what stops a replay from
// sending one vendor's thinking blocks to another.
func (m Message) rawOf(provider string) any {
	if m.rawProvider != provider {
		return nil
	}

	return m.rawTurn
}

// wireMessage is what a Message looks like once stored: the fields everyone
// shares, and the recorded turn as the bytes its own provider wrote.
type wireMessage struct {
	Role        string          `json:"role"`
	Text        string          `json:"text,omitempty"`
	ToolCalls   []ToolCall      `json:"tool_calls,omitempty"`
	ToolResults []ToolResult    `json:"tool_results,omitempty"`
	RawProvider string          `json:"raw_provider,omitempty"`
	Raw         json.RawMessage `json:"raw,omitempty"`
}

// MarshalJSON encodes the message with the recorded turn alongside it.
func (m Message) MarshalJSON() ([]byte, error) {
	wire := wireMessage{
		Role:        m.Role,
		Text:        m.Text,
		ToolCalls:   m.ToolCalls,
		ToolResults: m.ToolResults,
		RawProvider: m.rawProvider,
	}

	if m.rawTurn != nil {
		encoded, err := json.Marshal(m.rawTurn)
		if err != nil {
			return nil, fmt.Errorf("cannot encode the %s turn: %w", m.rawProvider, err)
		}
		wire.Raw = encoded
	}

	return json.Marshal(wire)
}

// UnmarshalJSON reads a stored message back, handing the recorded turn to the
// provider that wrote it.
//
// A turn this build cannot read - no codec registered, or bytes that no longer
// fit the provider's type - is kept as the bytes it arrived as rather than
// raised. Replay falls back to rebuilding the turn from the shared fields, so
// the run resumes having lost a thought signature; refusing the whole message
// would lose the conversation instead. Storing it again passes those same
// bytes through untouched, for a build that does know the provider.
func (m *Message) UnmarshalJSON(data []byte) error {
	var wire wireMessage
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}

	*m = Message{
		Role:        wire.Role,
		Text:        wire.Text,
		ToolCalls:   wire.ToolCalls,
		ToolResults: wire.ToolResults,
		rawProvider: wire.RawProvider,
	}

	if len(wire.Raw) == 0 {
		return nil
	}

	decode, known := rawCodecs[wire.RawProvider]
	if !known {
		m.rawTurn = wire.Raw
		return nil
	}

	turn, err := decode(wire.Raw)
	if err != nil {
		m.rawTurn = wire.Raw
		return nil
	}

	m.rawTurn = turn

	return nil
}
