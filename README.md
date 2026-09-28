# justsay

Agent Harness written in Go.

## Run the project

1. Install the Go version declared in [go.mod](go.mod) (currently Go 1.26.5), then confirm it is available:

   ```bash
   go version
   ```

2. Create a local environment file and fill in the required values. `.env` is read by the application and should not be committed.

   ```bash
   cp .env.example .env
   ```

   At a minimum, set:

   ```dotenv
   GEMINI_API_KEY="your Gemini API key"
   GEMINI_MODEL="your Gemini model ID"
   MOCK_AGENT_CALL="false"
   ```

   Set `MOCK_AGENT_CALL` to `true` to start without making model requests. Set `HITL_ENABLED` to `true` to hold the tool calls listed in `agent/human-in-the-loop/hitl_config.yml` for human approval. The example `.env` uses in-memory state, so approvals, including those sent through a paired Telegram account, work without Redis while the process is running.

3. Run the test suite from the repository root:

   ```bash
   go test ./...
   ```

4. Start the CLI from the repository root:

   ```bash
   go run .
   ```

## Agent state storage

Select a backend in `.env` with `HITL_STATE_STORE="inmemory"` or
`HITL_STATE_STORE="redis"`. Alternatively, create `justsay-config.yml` in the
repository root:

```yaml
state:
  backend: inmemory
```

For Redis:

```yaml
state:
  backend: redis
```

`state.backend` takes precedence over `HITL_STATE_STORE`. If neither is set,
the backend defaults to `redis` to preserve existing deployments. Unknown
backends cause an error. The agent opens its store on the first pause or
approval and retains it until shutdown; restart to apply configuration changes.

The `inmemory` backend requires no Redis connection or configuration. State is
local to the running agent and is lost when it closes or the process restarts.
Use `redis` for persistent state or state shared across instances. Redis requires
`REDIS_ADDR` in `.env`; `REDIS_PASSWORD`, `REDIS_DB` (a numeric database index),
and `REDIS_KEY_PREFIX` configure authentication, database, and key namespace.

Both backends use `HITL_STATE_TTL` from `.env` (default `24h`). Each save restarts
the expiry; `HITL_STATE_TTL="0"` disables it. In-memory entries expire on read
and are also cleaned up periodically during writes.

## CLI commands

On startup, the terminal shows a panel with the selected model and saved
integration status: Gmail authorization, Telegram pairing, and GitHub setup
(currently not configured). Status is read locally; it is not a live service
health check. Type `help` for integration setup commands. Narrow
terminals use a compact layout, and `NO_COLOR` disables panel colors.

| Command | Description |
| --- | --- |
| `help` | Show built-in and integration commands. |
| `reset` | Clear the current conversation. |
| `/on` | Enable real model calls (`MOCK_AGENT_CALL=false`). |
| `/off` | Mock model calls (`MOCK_AGENT_CALL=true`). |
| `/verify telegram` | Connect a Telegram bot to the running agent. |
| `/verify gmail` | Authorize Gmail sending with Google OAuth 2.0. |
| `exit` | Close the CLI. |

## Telegram integration

Run `/verify telegram` at the `justsay>` prompt. Enter a Telegram bot token when prompted (create one with [@BotFather](https://t.me/BotFather) if needed). The CLI validates the token, prints a one-time `/verify <code>` command, and waits for you to send it to that bot from the Telegram account to pair. After pairing, the CLI keeps listening for messages from that account and restores the saved pairing when it starts again.

## Gmail integration

The Gmail integration uses Google OAuth 2.0 and requests only
`https://www.googleapis.com/auth/gmail.send`. It can send plain-text messages
with `to`, `subject`, `body`, and optional `cc` and `bcc` fields. It cannot read,
search, delete, or label mail.

### Google Cloud setup

1. In the [Google Cloud console](https://console.cloud.google.com/), create or
   select a project and enable the **Gmail API**.
2. Configure the OAuth consent screen. While the app is in testing, add the
   Gmail accounts that will authorize it as test users.
3. Create an OAuth client ID with application type **Desktop app**.
4. Put the client values in the local `.env` file:

   ```dotenv
   GOOGLE_OAUTH_CLIENT_ID="your desktop OAuth client ID"
   GOOGLE_OAUTH_CLIENT_SECRET="your desktop OAuth client secret"
   ```

5. Start justsay and run `/verify gmail`. Open the printed Google authorization
   URL in a browser on the same machine. Google redirects to a temporary
   loopback listener; after the terminal reports `gmail: connected`, the
   listener closes.

You can then ask, for example:

```text
Send an email to john@example.com with the subject "Meeting Update" saying
that the meeting has been moved to 3 PM.
```

The agent exposes this operation as `gmail_send`. When human-in-the-loop mode
is enabled, it is held for approval before sending because email is an external
side effect.

Access and refresh tokens are stored in the ignored `credentials.json` file,
which is written with owner-only (`0600`) permissions. Expired access tokens
are refreshed automatically and the replacement is persisted. OAuth secrets,
tokens, and the one-time authorization URL are not included in model tool
arguments, model context, or session transcripts. Do not commit `.env` or
`credentials.json`.
