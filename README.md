# agent-space

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

   Set `MOCK_AGENT_CALL` to `true` to start without making model requests. Configure the `REDIS_*` values for human approvals, including approvals sent through a paired Telegram account, since paused approvals are stored in Redis.

3. Start the CLI from the repository root:

   ```bash
   go run .
   ```

## HTTP API

Start agent-space in HTTP mode by supplying a listen address. Use a loopback
address unless you have added authentication in front of the service.

```bash
go run . -http 127.0.0.1:8080
```

Invoke the same agent execution flow used by the CLI:

```bash
curl -X POST http://127.0.0.1:8080/agent/invoke \
  -H 'Content-Type: application/json' \
  -d '{"input":"Summarize the current directory"}'
```

A successful request returns `{"success":true,"result":"..."}`. Invalid
input returns HTTP 400, and agent execution failures return HTTP 500 with
`{"success":false,"error":"..."}`.

## CLI commands

| Command | Description |
| --- | --- |
| `help` | Show built-in and integration commands. |
| `reset` | Clear the current conversation. |
| `/on` | Enable real model calls (`MOCK_AGENT_CALL=false`). |
| `/off` | Mock model calls (`MOCK_AGENT_CALL=true`). |
| `/verify telegram` | Connect a Telegram bot to the running agent. |
| `exit` | Close the CLI. |

## Telegram integration

Run `/verify telegram` at the `agent-space>` prompt. Enter a Telegram bot token when prompted (create one with [@BotFather](https://t.me/BotFather) if needed). The CLI validates the token, prints a one-time `/verify <code>` command, and waits for you to send it to that bot from the Telegram account to pair. After pairing, the CLI keeps listening for messages from that account and restores the saved pairing when it starts again.
