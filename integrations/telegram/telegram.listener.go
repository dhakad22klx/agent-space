package telegram

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	// retryDelay is the first pause after a failed poll, doubling up to
	// maxRetryDelay. A dropped wifi connection is the common case and it comes
	// back on its own; giving up on the first failure would mean the link dies
	// whenever the laptop changes network.
	retryDelay = 2 * time.Second

	// maxRetryDelay caps the backoff, so a long outage still reconnects within
	// a minute of coming back rather than an hour later.
	maxRetryDelay = time.Minute
)

// Handler answers one request from the paired chat. It is supplied by the
// caller, which is what keeps this package from knowing anything about the agent
// or the model: this file's job is deciding whose messages are worth running,
// not running them.
//
// An error is a failed request, and the listener reports it in the chat. A
// handler is not expected to be quick — the poll waits for it, so the paired
// user cannot get two requests interleaved.
type Handler func(ctx context.Context, text string) (string, error)

// Listener is the running link to Telegram: it reads messages from the one
// paired chat, hands them to the handler, and sends the outcome back.
type Listener struct {
	// Client speaks to the bot the pairing was made with.
	Client *Client

	// ChatID is the only chat that may give this agent orders.
	ChatID int64

	// Handle runs an authorised request.
	Handle Handler

	// Approve acts on an authorised tap of an approval button.
	Approve Approvals

	// Trace, when set, reports what the link is doing to whoever is watching
	// the terminal. It never receives message text from an unpaired chat.
	Trace func(string)
}

// Run polls until ctx is cancelled, which is how the CLI stops the link when
// the user leaves or re-pairs.
//
// A returned error is one this cannot recover from — a token Telegram no longer
// accepts, a second poller on the same bot. Anything else is retried, because a
// local agent's network comes and goes and the link should outlive that.
func (l *Listener) Run(ctx context.Context) error {
	backoff := retryDelay

	var offset int64
	listening := false

	for {
		// Establishing the cursor is retried like any other request. An agent
		// started before the network is up should join the moment it arrives,
		// rather than sit dead until someone restarts it.
		if !listening {
			start, err := l.Client.SkipBacklog(ctx)
			if err != nil {
				if stop, fatal := l.recover(ctx, err, &backoff); stop {
					return fatal
				}

				continue
			}

			offset, listening = start, true
			backoff = retryDelay
			l.trace(fmt.Sprintf("telegram: listening for chat %d", l.ChatID))
		}

		updates, err := l.Client.GetUpdates(ctx, offset, PollWait)
		if err != nil {
			if stop, fatal := l.recover(ctx, err, &backoff); stop {
				return fatal
			}

			continue
		}

		backoff = retryDelay

		for _, update := range updates {
			// The cursor moves before the work is done, so a request that fails
			// or takes a long time is not redelivered and run twice. For an
			// agent that edits files, running something twice is worse than
			// losing it.
			offset = update.ID + 1

			l.dispatch(ctx, update)
		}
	}
}

// recover decides what a failed request means. It reports whether to stop, and
// with which error; a false means try again, and the pause has already been
// waited out by the time it returns.
//
// Three cases, and they are genuinely different. Leaving is not a failure: the
// cancelled request is how leaving arrives here. A rejected token or a second
// poller cannot be retried into working, so the link closes and says why.
// Everything else is a network that will probably come back.
func (l *Listener) recover(ctx context.Context, err error, backoff *time.Duration) (stop bool, fatal error) {
	if ctx.Err() != nil {
		return true, nil
	}

	if terminal := terminal(err); terminal != nil {
		return true, terminal
	}

	l.trace("telegram: " + err.Error() + " — retrying in " + backoff.String())

	if !sleep(ctx, *backoff) {
		return true, nil
	}

	if *backoff *= 2; *backoff > maxRetryDelay {
		*backoff = maxRetryDelay
	}

	return false, nil
}

// dispatch decides what to do with one update.
func (l *Listener) dispatch(ctx context.Context, update Update) {
	// A tap is not a request: it answers one the agent already asked about.
	if update.CallbackQuery != nil {
		l.decide(ctx, update.CallbackQuery)
		return
	}

	message := update.Message
	if message == nil {
		return
	}

	text := strings.TrimSpace(message.Text)
	if text == "" {
		return
	}

	if message.Chat.ID != l.ChatID {
		// Ignored in silence, on purpose. A reply would confirm to a stranger
		// that the bot is live and would hand them a way to have this machine
		// send messages; the paired user learns nothing from it either. The
		// local trace is where it is visible, and the text is not repeated
		// there, since it came from someone with no standing here.
		l.trace(fmt.Sprintf("telegram: ignored a message from unauthorized chat %d", message.Chat.ID))
		return
	}

	// /start is what Telegram sends when someone opens a bot for the first
	// time. It is a greeting rather than a request, and answering it here keeps
	// the agent from being asked to interpret it.
	if Command(text) == "/start" {
		l.reply(ctx, "Paired. Send a request in plain text — for example: what changed in this repo today?")
		return
	}

	l.trace("telegram ← " + summarize(text))

	reply, err := l.Handle(ctx, text)
	if err != nil {
		// The failure is the answer, and the person who asked is not at this
		// terminal to read it, so it goes to the chat rather than only here.
		l.reply(ctx, "failed: "+err.Error())
		l.trace("telegram: request failed — " + err.Error())
		return
	}

	l.reply(ctx, reply)
}

// decide acts on a tap of an approval button.
func (l *Listener) decide(ctx context.Context, query *CallbackQuery) {
	// A tap is authorised on the chat its message lives in, the same gate
	// messages get, and ignored in silence for the same reason.
	if query.Message == nil || query.Message.Chat.ID != l.ChatID {
		l.trace("telegram: ignored a button tap from an unauthorized chat")
		return
	}

	decision, sessionID, approvalID, ok := ParseApproval(query.Data)
	if !ok {
		l.answer(ctx, query.ID, "")
		l.trace("telegram: ignored a button this build does not understand")
		return
	}

	// Answered before the work: Telegram expires a query id in seconds, and an
	// approved run takes as long as the model does.
	l.answer(ctx, query.ID, "working on it…")

	// The buttons go before the decision runs, so a second tap has nothing left
	// to hit. An already-stripped message is refused here, which is fine.
	if err := l.Client.StripButtons(ctx, query.Message.Chat.ID, query.Message.ID); err != nil {
		l.trace("telegram: could not clear the buttons — " + err.Error())
	}

	if l.Approve == nil {
		l.reply(ctx, "this agent cannot act on approvals")
		return
	}

	l.trace(fmt.Sprintf("telegram ← %s for session %s", decision, sessionID))

	reply, err := l.Approve(ctx, decision, sessionID, approvalID)
	if err != nil {
		l.reply(ctx, "failed: "+err.Error())
		l.trace("telegram: the decision failed — " + err.Error())
		return
	}

	l.reply(ctx, reply)
}

// answer clears the spinner on a tapped button.
func (l *Listener) answer(ctx context.Context, queryID, notice string) {
	if err := l.Client.AnswerCallbackQuery(ctx, queryID, notice); err != nil {
		if ctx.Err() != nil {
			return
		}

		l.trace("telegram: could not answer the button tap — " + err.Error())
	}
}

// reply sends text to the paired chat, reporting a send that failed to the
// terminal instead of losing it quietly.
func (l *Listener) reply(ctx context.Context, text string) {
	if err := l.Client.SendMessage(ctx, l.ChatID, text); err != nil {
		if ctx.Err() != nil {
			return
		}

		l.trace("telegram: could not send the reply — " + err.Error())
	}
}

func (l *Listener) trace(line string) {
	if l.Trace != nil {
		l.Trace(line)
	}
}

// terminal returns the error to stop on, or nil when the failure is worth
// retrying. Waiting cannot fix a token Telegram rejects, and it cannot fix two
// pollers on one bot — retrying either just repeats the refusal forever, so
// both are reported and the link closes.
func terminal(err error) error {
	var refusal *Refusal
	if !errors.As(err, &refusal) {
		return nil
	}

	switch {
	case refusal.Unauthorized():
		return fmt.Errorf("telegram no longer accepts this bot token: %w", err)
	case refusal.Conflict():
		return fmt.Errorf("another poller is already connected to this bot: %w", err)
	}

	return nil
}

// Command is the slash-command a message opens with, lowercased and with the
// @botname a group chat appends stripped off. It is empty for ordinary text,
// which is what most requests are.
func Command(text string) string {
	fields := strings.Fields(text)
	if len(fields) == 0 || !strings.HasPrefix(fields[0], "/") {
		return ""
	}

	name := strings.ToLower(fields[0])
	if bare, _, found := strings.Cut(name, "@"); found {
		name = bare
	}

	return name
}

// summarize shortens a message for the terminal trace, which is a line saying
// what arrived rather than a copy of it.
func summarize(text string) string {
	const width = 80

	line := text
	if first, _, found := strings.Cut(line, "\n"); found {
		line = first + " …"
	}

	runes := []rune(line)
	if len(runes) > width {
		line = string(runes[:width]) + "…"
	}

	return line
}

// sleep waits for d, or returns false as soon as ctx is done. A plain
// time.Sleep here would keep a cancelled listener alive for the whole backoff.
func sleep(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
