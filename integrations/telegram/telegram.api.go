// Package telegram connects the agent to a Telegram bot: it checks a bot token,
// pairs the bot with one chat over a one-time code, and then takes plain-text
// requests from that chat alone.
//
// Nothing here writes to the terminal or reads from it. The pairing flow is
// handed a callback to announce its code with, which leaves the CLI as the only
// thing that talks to the user and lets this package be tested without a
// terminal at all.
package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	// PollWait is how long Telegram holds a getUpdates request open while
	// nothing is happening. Long polling is what lets the agent run behind a
	// laptop's NAT with no public URL, and a long hold is the point: it is one
	// idle request every half minute instead of a busy loop.
	PollWait = 30 * time.Second

	// httpTimeout has to outlast a held-open poll, or every quiet stretch would
	// look like a network failure. The margin covers a slow reply to the
	// request Telegram answers right at its own deadline.
	httpTimeout = PollWait + 30*time.Second

	// maxResponseBytes bounds what one reply may be. A backlog of updates is
	// the large case, and this is far above it.
	maxResponseBytes = 4 << 20

	// MaxMessageRunes is the Bot API's own limit on the text of one message.
	MaxMessageRunes = 4096
)

// api is the Bot API host. The token goes in the path here, not in a header,
// which is why nothing in this package may print a URL.
const api = "https://api.telegram.org"

// Client talks to one bot's Bot API.
//
// Every method funnels through call, which is the only place the token is
// interpolated and the only place an error is built. That is deliberate: the
// Bot API puts the token in the URL path, so an unwrapped transport error —
// which quotes the URL it failed on — is the token written to the terminal.
type Client struct {
	token string

	// API is the Bot API host requests are sent to. It defaults to the real
	// one; it is a field rather than a constant so a test can point a client at
	// a local server.
	API string

	http *http.Client
}

// NewClient builds a client for the given bot token.
func NewClient(token string) *Client {
	return &Client{
		token: strings.TrimSpace(token),
		API:   api,
		http:  &http.Client{Timeout: httpTimeout},
	}
}

// User is a Telegram account: the bot itself, as getMe reports it, or the
// sender of a message.
type User struct {
	ID                    int64  `json:"id"`
	IsBot                 bool   `json:"is_bot"`
	FirstName             string `json:"first_name"`
	Username              string `json:"username"`
	CanJoinGroups         bool   `json:"can_join_groups"`
	SupportsInlineQueries bool   `json:"supports_inline_queries"`
}

// Chat is where a message arrived. Its ID is what the agent is paired to and
// what a reply is addressed to; for a direct conversation with one person it is
// that person's own chat.
type Chat struct {
	ID       int64  `json:"id"`
	Type     string `json:"type"`
	Username string `json:"username"`
}

// Message is one message in a chat. Only text is read: this integration takes
// plain-text requests, so a photo or a sticker is an update with nothing in it
// worth acting on.
type Message struct {
	ID   int64  `json:"message_id"`
	From *User  `json:"from"`
	Chat Chat   `json:"chat"`
	Text string `json:"text"`
	Date int64  `json:"date"`
}

// Button is one inline-keyboard button: the label the user reads, and the token
// that comes back when it is pressed.
//
// Data is the whole of what a press tells us — Telegram sends it back and
// nothing else — so it has to identify what was being decided as well as the
// answer. The Bot API caps it at 64 bytes, which is why it is a token rather
// than a sentence.
type Button struct {
	Label string `json:"text"`
	Data  string `json:"callback_data"`
}

// keyboard is the reply_markup a message's buttons travel in. Buttons are laid
// out in rows; one row is all this sends, since two choices side by side is the
// whole of what it offers.
type keyboard struct {
	Rows [][]Button `json:"inline_keyboard"`
}

// CallbackQuery is a button press.
//
// Message is the message the button was attached to, which is what makes the
// press meaningful: the answer is "approve", and the question is whatever that
// message said. Answering the press is not optional — Telegram spins the button
// until it is acknowledged — which is what AnswerCallback is for.
type CallbackQuery struct {
	ID      string   `json:"id"`
	From    *User    `json:"from"`
	Message *Message `json:"message"`
	Data    string   `json:"data"`
}

// Update is one thing that happened, as getUpdates reports it. ID is the cursor:
// asking again from ID+1 is what tells Telegram this one was dealt with.
//
// Exactly one of the fields below is set on any update this asks for: a message
// that arrived, or a button that was pressed.
type Update struct {
	ID            int64          `json:"update_id"`
	Message       *Message       `json:"message"`
	CallbackQuery *CallbackQuery `json:"callback_query"`
}

// Refusal is the Bot API answering that it understood the request and will not
// do it: a token it does not accept, a chat the bot cannot write to, a second
// poller on the same bot. It is a verdict about the request, so it travels as a
// distinct type rather than as a generic error — the caller reacts differently
// to "your token is wrong" than to "the network is down".
type Refusal struct {
	Method      string
	Code        int
	Description string
}

func (r *Refusal) Error() string {
	// Telegram explains the problem better than a paraphrase would, so the
	// description is quoted as it arrived.
	if r.Description == "" {
		return fmt.Sprintf("the Bot API refused %s with code %d", r.Method, r.Code)
	}

	return fmt.Sprintf("the Bot API refused %s: %s", r.Method, r.Description)
}

// Unauthorized reports whether Telegram rejected the token itself. It is worth
// separating because it cannot be waited out: retrying with the same token will
// be refused the same way forever.
func (r *Refusal) Unauthorized() bool { return r.Code == http.StatusUnauthorized }

// Conflict reports whether something else is already long-polling this bot.
// Telegram allows one getUpdates at a time, so this means a second agent — or a
// leftover poller — is holding the connection.
func (r *Refusal) Conflict() bool { return r.Code == http.StatusConflict }

// envelope is the wrapper every Bot API reply arrives in.
type envelope struct {
	OK          bool            `json:"ok"`
	Description string          `json:"description"`
	ErrorCode   int             `json:"error_code"`
	Result      json.RawMessage `json:"result"`
}

// GetMe looks up the bot the token controls: the cheapest request that succeeds
// only for a token Telegram accepts.
func (c *Client) GetMe(ctx context.Context) (User, error) {
	var me User
	err := c.call(ctx, "getMe", nil, &me)

	return me, err
}

// GetUpdates waits up to wait for something to happen, starting from offset.
//
// Offset is a confirmation as much as a cursor: asking from N tells Telegram
// every update before N was handled, and it stops resending them.
func (c *Client) GetUpdates(ctx context.Context, offset int64, wait time.Duration) ([]Update, error) {
	params := url.Values{}
	params.Set("offset", strconv.FormatInt(offset, 10))
	params.Set("timeout", strconv.Itoa(int(wait.Seconds())))
	// Nothing else is acted on, so nothing else is worth being sent or having
	// to skip past. Button presses are asked for here rather than only while
	// listening, because this is the one place updates are requested; pairing
	// shares it and skips anything that is not a message.
	params.Set("allowed_updates", `["message","callback_query"]`)

	var updates []Update
	if err := c.call(ctx, "getUpdates", params, &updates); err != nil {
		return nil, err
	}

	return updates, nil
}

// SkipBacklog throws away whatever Telegram has been holding and returns the
// offset to start from.
//
// This runs before the agent listens, and it matters: Telegram keeps
// undelivered messages for a day, so without it an agent started in the evening
// would wake up and run every command sent while it was off — in order, with no
// one watching. A message worth acting on is one sent to a listening agent.
func (c *Client) SkipBacklog(ctx context.Context) (int64, error) {
	// -1 asks for the most recent update only, without confirming anything; the
	// next call, made from the offset returned here, is what discards the rest.
	updates, err := c.GetUpdates(ctx, -1, 0)
	if err != nil {
		return 0, err
	}

	if len(updates) == 0 {
		return 0, nil
	}

	return updates[len(updates)-1].ID + 1, nil
}

// SendMessage writes one message to a chat, with the given buttons under it.
//
// Two things happen to the text on the way out. The token is scrubbed from it,
// because this is the one place anything leaves the machine and the agent can
// be asked to read files; and it is clamped to the Bot API's length limit,
// because an over-long message is refused outright rather than shortened.
//
// The buttons are variadic so that the messages which want none — a nudge
// during pairing, a failure — read exactly as they did before there were any.
func (c *Client) SendMessage(ctx context.Context, chatID int64, text string, buttons ...Button) error {
	body := Clamp(c.Scrub(text))

	// Telegram rejects an empty message, and a command that printed nothing is
	// a real outcome that deserves to be reported rather than dropped.
	if strings.TrimSpace(body) == "" {
		body = "(no output)"
	}

	params := url.Values{}
	params.Set("chat_id", strconv.FormatInt(chatID, 10))
	params.Set("text", body)
	// No parse_mode on purpose. The agent's replies carry shell output, paths
	// and code, where an unpaired _ or * is ordinary; asking Telegram to read
	// that as Markdown would have it refuse the message over punctuation.

	if len(buttons) > 0 {
		markup, err := json.Marshal(keyboard{Rows: [][]Button{buttons}})
		if err != nil {
			return fmt.Errorf("cannot build the buttons for sendMessage: %w", err)
		}

		params.Set("reply_markup", string(markup))
	}

	return c.call(ctx, "sendMessage", params, nil)
}

// AnswerCallback acknowledges a button press, with an optional line shown to the
// user as a toast over the chat.
//
// It is not a courtesy. Telegram leaves a pressed button spinning until the
// press is answered, so a link that acts on presses without calling this looks
// broken from the phone even when it worked.
func (c *Client) AnswerCallback(ctx context.Context, queryID, notice string) error {
	params := url.Values{}
	params.Set("callback_query_id", queryID)
	if notice != "" {
		params.Set("text", c.Scrub(notice))
	}

	return c.call(ctx, "answerCallbackQuery", params, nil)
}

// EditMessage replaces the text of a message already sent, and takes its
// buttons away with it: a reply_markup that is not sent is a keyboard removed.
//
// That is what settles a decision. The choice made is written into the message
// it was made about, and the buttons stop being there to press again.
func (c *Client) EditMessage(ctx context.Context, chatID, messageID int64, text string) error {
	params := url.Values{}
	params.Set("chat_id", strconv.FormatInt(chatID, 10))
	params.Set("message_id", strconv.FormatInt(messageID, 10))
	params.Set("text", Clamp(c.Scrub(text)))

	return c.call(ctx, "editMessageText", params, nil)
}

// Clamp shortens text to what the Bot API will accept, keeping the beginning,
// which is where a command says what it did. The notice is counted inside the
// limit rather than added past it.
func Clamp(text string) string {
	runes := []rune(text)
	if len(runes) <= MaxMessageRunes {
		return text
	}

	const notice = "\n…[truncated]"
	keep := MaxMessageRunes - len([]rune(notice))

	return string(runes[:keep]) + notice
}

// call makes one Bot API request and decodes its result into out, which may be
// nil when the reply carries nothing worth reading.
func (c *Client) call(ctx context.Context, method string, params url.Values, out any) error {
	if c.token == "" {
		return fmt.Errorf("cannot call %s: no bot token", method)
	}

	// The token goes in the path, so it is escaped: a stray slash inside it
	// would otherwise change which method is being called.
	endpoint := c.API + "/bot" + url.PathEscape(c.token) + "/" + method

	if params == nil {
		params = url.Values{}
	}

	// The arguments go in the body rather than the query string. The token is
	// already in the path and cannot move, but nothing else needs to be in a
	// URL that might be logged by something in between.
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(params.Encode()))
	if err != nil {
		return fmt.Errorf("cannot build the %s request: %s", method, c.Scrub(err.Error()))
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	response, err := c.http.Do(request)
	if err != nil {
		// An http error quotes the URL it failed on, and the URL holds the
		// token. This is the reason Scrub exists.
		return fmt.Errorf("cannot reach the Bot API for %s: %s", method, c.Scrub(err.Error()))
	}
	defer response.Body.Close()

	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes))
	if err != nil {
		return fmt.Errorf("cannot read the reply to %s: %s", method, c.Scrub(err.Error()))
	}

	var reply envelope
	if err := json.Unmarshal(body, &reply); err != nil {
		// Something answered, but not the Bot API: a captive portal, a proxy,
		// an api host pointed somewhere else. The body is not quoted, since
		// whatever it is was not built with a secret in mind.
		return fmt.Errorf("the reply to %s was not Bot API JSON (HTTP %d)", method, response.StatusCode)
	}

	if !reply.OK {
		return &Refusal{Method: method, Code: reply.ErrorCode, Description: reply.Description}
	}

	if out == nil {
		return nil
	}

	if err := json.Unmarshal(reply.Result, out); err != nil {
		return fmt.Errorf("cannot read the result of %s: %w", method, err)
	}

	return nil
}

// Scrub replaces the token wherever it appears in text.
//
// Both spellings are covered: the token as it was typed, and the form that ends
// up in a URL. They are usually the same string — a colon and the token's
// alphabet all survive path escaping — but relying on that would make this
// silently wrong the day it stops being true.
func (c *Client) Scrub(text string) string {
	if c.token == "" {
		return text
	}

	const placeholder = "<token>"

	for _, form := range []string{c.token, url.PathEscape(c.token), url.QueryEscape(c.token)} {
		text = strings.ReplaceAll(text, form, placeholder)
	}

	// The half after the colon is the part that is actually secret; the bot id
	// in front of it is public. Replacing it on its own catches anything that
	// split the token before quoting it.
	if secret := c.token[strings.Index(c.token, ":")+1:]; len(secret) > 8 {
		text = strings.ReplaceAll(text, secret, placeholder)
	}

	return text
}
