package telegram

import (
	"context"
	"strings"
)

// The decisions a button can carry. Strings, so a payload read off the wire
// says what it means.
const (
	DecisionApprove = "approve"
	DecisionDecline = "decline"
)

// tagLen is how much of a pause's id a button carries. The whole thing would
// not fit beside the session id inside MaxCallbackDataBytes.
const tagLen = 8

// Approvals answers a tap on an approval button. Injected for the same reason
// Handle is: this package decides who may answer, not what an answer does.
type Approvals func(ctx context.Context, decision, sessionID, approvalID string) (string, error)

// ApprovalData builds a button's payload. The tag tells one pause of a session
// from another, so an older message cannot approve whatever is waiting now.
func ApprovalData(decision, sessionID, approvalID string) string {
	tag := approvalID
	if len(tag) > tagLen {
		tag = tag[:tagLen]
	}

	return decision + ":" + sessionID + ":" + tag
}

// ParseApproval reads that payload back. Not ok means the button is not ours.
func ParseApproval(data string) (decision, sessionID, approvalID string, ok bool) {
	decision, rest, found := strings.Cut(data, ":")
	if !found || (decision != DecisionApprove && decision != DecisionDecline) {
		return "", "", "", false
	}

	// A payload minted before the tag existed still works: the guard reading it
	// treats an empty tag as "none sent".
	sessionID, approvalID, _ = strings.Cut(rest, ":")
	if sessionID == "" {
		return "", "", "", false
	}

	return decision, sessionID, approvalID, true
}
