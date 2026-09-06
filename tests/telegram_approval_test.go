package tests

import (
	"testing"

	telegram "agent-harness/integrations/telegram"
)

// TestApprovalDataFits pins the Bot API's 64-byte limit on a button payload.
// A longer decision name or a wider tag would break it silently otherwise.
func TestApprovalDataFits(t *testing.T) {
	const (
		session = "9f3ca21b-4e77-4c1a-9c3d-2b8e1a0f4d21"
		pending = "7e4b1a02-1111-2222-3333-444455556666"
	)

	for _, decision := range []string{telegram.DecisionApprove, telegram.DecisionDecline} {
		data := telegram.ApprovalData(decision, session, pending)
		if len(data) > telegram.MaxCallbackDataBytes {
			t.Fatalf("%q is %d bytes, over the %d allowed", data, len(data), telegram.MaxCallbackDataBytes)
		}

		gotDecision, gotSession, gotTag, ok := telegram.ParseApproval(data)
		if !ok || gotDecision != decision || gotSession != session || gotTag != pending[:8] {
			t.Fatalf("round trip gave %q %q %q (ok %v)", gotDecision, gotSession, gotTag, ok)
		}
	}
}

// TestParseApprovalRejects covers the payloads a tap must not be acted on.
func TestParseApprovalRejects(t *testing.T) {
	for _, data := range []string{"", "nope:session", "approve:", "approve", ":::"} {
		if _, _, _, ok := telegram.ParseApproval(data); ok {
			t.Errorf("%q should not parse", data)
		}
	}
}

// A payload minted before the tag existed still names its session.
func TestParseApprovalWithoutTag(t *testing.T) {
	const session = "9f3ca21b-4e77-4c1a-9c3d-2b8e1a0f4d21"

	decision, got, tag, ok := telegram.ParseApproval(telegram.DecisionApprove + ":" + session)
	if !ok || decision != telegram.DecisionApprove || got != session || tag != "" {
		t.Fatalf("gave %q %q %q (ok %v)", decision, got, tag, ok)
	}
}
