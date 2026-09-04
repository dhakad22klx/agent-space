package tools

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
)

// SendUpdatesToManager pretends to pass a progress update along to the user's
// manager. Nothing leaves the machine: it logs the update and acknowledges it.
//
// It exists as a target for the human-in-the-loop work, which needs a tool that
// is worth pausing for and cheap to approve, reject or replay. RunBash is the
// wrong place to try that out, since a mistake there changes a tool the agent
// already depends on.
type SendUpdatesToManager struct {
	// Manager names who the update is addressed to, for the log line and the
	// acknowledgement. Empty means defaultManager.
	Manager string
}

// defaultManager stands in until a real recipient is configured.
const defaultManager = "manager"

// NewSendUpdatesToManager reports to the placeholder manager.
func NewSendUpdatesToManager() *SendUpdatesToManager {
	return &SendUpdatesToManager{Manager: defaultManager}
}

// Schema tells the model when and how to call this tool.
func (s *SendUpdatesToManager) Schema() Schema {
	return Schema{
		Name: "send_updates_to_manager",
		Description: "Send a short progress update to the user's manager. " +
			"Use it when the user asks to report status, flag a blocker, or let their manager know " +
			"where something stands. " +
			"The update is delivered as written, so say what happened in a sentence or two rather than " +
			"pasting raw command output. " +
			"This tool only sends; it cannot read a reply, so do not wait for one.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"update": map[string]any{
					"type":        "string",
					"description": `The update to send, for example "Finished the migration script; deploying tomorrow."`,
				},
			},
			"required": []string{"update"},
		},
	}
}

// Call logs the update and confirms it. There is no delivery yet, so the
// confirmation says the update was recorded rather than claiming it was sent;
// the model would otherwise tell the user their manager has been told.
func (s *SendUpdatesToManager) Call(ctx context.Context, args map[string]any) (string, error) {
	update, _ := args["update"].(string)
	update = strings.TrimSpace(update)
	if update == "" {
		return "", errors.New(`argument "update" is required`)
	}

	manager := s.Manager
	if manager == "" {
		manager = defaultManager
	}

	// stderr, where log writes by default, keeps this out of the answer the TUI
	// is painting on stdout.
	log.Printf("send_updates_to_manager: to=%q update=%q", manager, update)

	return fmt.Sprintf("Recorded this update for %s (delivery is not wired up yet): %s", manager, update), nil
}
