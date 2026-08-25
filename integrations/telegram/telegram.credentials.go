package telegram

import "time"

// CredentialsKey is this integration's entry in the credentials file.
const CredentialsKey = "telegram"

// Record is what has to survive a restart for the agent to trust Telegram
// again: the bot to speak as, and the one chat it will take orders from.
//
// The chat id is the whole of the authorisation. Telegram guarantees it
// identifies the conversation a message arrived in, and a stranger who finds
// the bot cannot forge it, so comparing against it is what keeps the agent from
// running anyone else's commands. The username beside it is only there to make
// the pairing legible to a person later — it can be changed by its owner, so
// nothing is decided on it.
type Record struct {
	BotToken           string `json:"bot_token"`
	AuthorizedChatID   int64  `json:"authorized_chat_id"`
	AuthorizedUsername string `json:"authorized_username,omitempty"`
	VerifiedAt         string `json:"verified_at"`
}

// Paired reports whether this record names a chat to listen to. A record with a
// token but no chat is a bot that was never handed to anyone.
func (r Record) Paired() bool { return r.AuthorizedChatID != 0 }

// NewRecord builds the record for a completed pairing, stamped in UTC so the
// file reads the same wherever it is opened.
func NewRecord(token string, who Identity, at time.Time) Record {
	return Record{
		BotToken:           token,
		AuthorizedChatID:   who.ChatID,
		AuthorizedUsername: who.Username,
		VerifiedAt:         at.UTC().Format(time.RFC3339),
	}
}
