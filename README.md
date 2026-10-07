# justsay

Agentic System, written in Go.

## Install

Install the latest published release with Go available:

```bash
curl -fsSL https://raw.githubusercontent.com/dhakad22klx/justsay/main/install.sh | sh
justsay
```

If the installer prints a PATH command, run it in your current shell or open a
new terminal before starting `justsay`. See [INSTALL.md](INSTALL.md) for supported
platforms, Gemini configuration, verification, updates, and removal.

## Run from source

1. Clone the repository and change into its directory:

   ```bash
   git clone https://github.com/dhakad22klx/justsay.git
   cd justsay
   ```

2. Install the Go version declared in [go.mod](go.mod) (currently Go 1.26.5), then confirm it is available:

   ```bash
   go version
   ```

3. Create a local environment file and fill in the required values. `.env` is read by the application and should not be committed.

   ```bash
   cp .env.example .env
   ```

   At a minimum, set:

   ```dotenv
   GEMINI_API_KEY="your Gemini API key"
   GEMINI_MODEL="your Gemini model ID"
   MOCK_AGENT_CALL="false"
   ```

   Set `MOCK_AGENT_CALL` to `true` to start without making model requests. Set `HITL_ENABLED` to `true` to hold the tool calls listed in `agent/human-in-the-loop/hitl_config.yml` for human approval. Both `inmemory` and `redis` support approvals, including those sent through a paired Telegram account. 
   The `inmemory` backend requires no Redis configuration and keeps state only while the agent is running. The `redis` backend requires Redis configuration and supports persistent state shared across instances. See [Agent state storage](#agent-state-storage) for configuration details.

4. Run the test suite from the repository root:

   ```bash
   go test ./...
   ```

5. Start the CLI from the repository root:

   ```bash
   go run .
   ```

## Agent state storage

Select a backend in `config.yml` in the repository root. The bundled
file selects `inmemory`:

```yaml
state:
  backend: inmemory
```

To select Redis, change that file to:

```yaml
state:
  backend: redis
```

To select through `.env` instead, remove `state.backend` from the YAML file
(or remove the file), then set `HITL_STATE_STORE="inmemory"` or
`HITL_STATE_STORE="redis"` in `.env`. A non-empty `state.backend` overrides
`.env`, so changing `.env` alone does not override the bundled configuration.

If neither is set, the backend defaults to `inmemory`. Select `redis` explicitly
to use Redis. Unknown backends cause an error, including an invalid
`HITL_STATE_STORE` when YAML overrides it. The agent opens its store on the first pause or
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

Process commands work with `go run . setup`, `go run . --help`, or the equivalent
installed `justsay` commands. Setup saves agent settings in `config.yml` and
secrets in the existing `credentials.json`, preserving integration credentials.
Provider settings use the generic `model_provider` entry, with `api_key` and
`model` saved together in `credentials.json`; `config.yml` stores only
`model_provider.name` (`gemini`). Previous provider-specific entries
are not used or migrated.
When HITL is enabled, setup asks for `inmemory` or `redis` (default `inmemory`,
or the saved backend on reconfiguration). Only `redis` prompts for connection
details.
Both files and session transcripts use the single `internal.ConfigDir` constant
in `internal/config.go`. The current value `.justsay` stores them in the user's
`~/.justsay/` directory, with transcripts in `~/.justsay/sessions/`, regardless of
where the CLI is launched. Other relative values also resolve under the user's
home directory; absolute paths are used directly. The value `.` uses the working
directory for local development. The directory is created on save or session
start, and credentials are readable only by their owner. Existing project-local
files are not moved automatically; run `justsay setup` to configure the selected
location.
Setup does not read, write, or move `.env`. Runtime prefers the model and API key
in `credentials.json`, falling back to `.env` for either missing value. The model
is never read from `config.yml`. Other agent settings retain their runtime `.env`
fallback until setup values are saved.

`justsay version` and `justsay update` are skeleton commands that return dummy
output; version tracking and binary updates will be implemented later.

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

## Integrations

Use `/verify telegram` to pair a Telegram bot or `/verify gmail` to authorize
Gmail sending. See [INTEGRATIONS.md](integrations/INTEGRATIONS.md) for setup
steps and integration details.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for the development and pull request
guidelines.
