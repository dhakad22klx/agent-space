package telegram

import (
	"context"
	"encoding/base64"
	"strings"

	"github.com/google/uuid"
)

// The decisions a button can carry. Strings, so a payload read off the wire
// says what it means.
const (
	DecisionApprove = "approve"
	DecisionDecline = "decline"
)

// DecisionSettled is what the buttons of a decided request are left answering
// with. Telegram has no disabled button, so an inert payload is what "inactive"
// is made of.
const DecisionSettled = "settled"

// Approvals answers a tap on an approval button. Injected for the same reason
// Handle is: this package decides who may answer, not what an answer does.
type Approvals func(ctx context.Context, decision, sessionID, approvalID string) (string, error)

// ApprovalData builds a button's payload: the decision, then the session and
// the pause it answers, both whole. The pause id is what keeps an older
// message from approving whatever is waiting now.
//
// Both ids are UUIDs, and two of them written out plainly are 81 bytes — past
// what a button may carry. Encoding the 16 bytes behind the 36 characters
// brings the pair to 45 and loses nothing: ParseApproval hands back the ids
// exactly as they went in.
func ApprovalData(decision, sessionID, approvalID string) string {
	return decision + ":" + packID(sessionID) + ":" + packID(approvalID)
}

// ParseApproval reads that payload back. Not ok means the button is not ours.
func ParseApproval(data string) (decision, sessionID, approvalID string, ok bool) {
	decision, rest, found := strings.Cut(data, ":")
	if !found || (decision != DecisionApprove && decision != DecisionDecline) {
		return "", "", "", false
	}

	// A payload minted before the pause id existed still works: the guard
	// reading it treats an empty id as "none sent".
	sessionID, approvalID, _ = strings.Cut(rest, ":")
	if sessionID == "" {
		return "", "", "", false
	}

	return decision, unpackID(sessionID), unpackID(approvalID), true
}

// SettledRow is the keyboard a decided request is left with: the button that
// was tapped is marked, and neither answers to anything.
//
// The mark says which button was pressed, not how the run went — that arrives
// as a message of its own.
func SettledRow(decision string) []Button {
	approve, decline := "Approve", "Decline"

	switch decision {
	case DecisionApprove:
		approve = "✅ " + approve + "d"
	case DecisionDecline:
		decline = "❌ " + decline + "d"
	}

	return []Button{
		{Text: approve, Data: DecisionSettled},
		{Text: decline, Data: DecisionSettled},
	}
}

// packID shortens a UUID to the bytes it stands for. Anything that is not one
// is carried as it stands, and the payload's length check has the last word.
func packID(id string) string {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return id
	}

	return base64.RawURLEncoding.EncodeToString(parsed[:])
}

// unpackID is packID backwards. A field that is not 16 encoded bytes was never
// packed — an id from an older build, or an empty one — and is returned as it
// arrived.
func unpackID(field string) string {
	raw, err := base64.RawURLEncoding.DecodeString(field)
	if err != nil || len(raw) != len(uuid.UUID{}) {
		return field
	}

	var id uuid.UUID
	copy(id[:], raw)

	return id.String()
}
