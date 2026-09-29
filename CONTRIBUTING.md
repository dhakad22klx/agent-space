# Contributing to justsay

Thanks for contributing. Bug reports, documentation improvements, tests, and
code changes are welcome.

## Get started

Follow the [README setup steps](README.md#run-the-project) to clone the project,
-install the Go version in `go.mod`, and configure a local `.env` file. 
- Install `golangci-lint` to run the lint check. 
Create a branch for your change:

```bash
git checkout -b feat/gmail-auth-token-ration
```

For integration changes, read [INTEGRATIONS.md](integrations/INTEGRATIONS.md)
before editing the integration handlers.

## Make a change

- Keep the change focused and explain why it is needed.
- Add or update tests when behavior changes.
- Do not commit `.env`, `credentials.json`, API keys, tokens, or session transcripts.

To apply available lint fixes or format Go code, run:

```bash
golangci-lint run --fix
golangci-lint fmt
```

For a single file, use `gofmt -w path/to/file.go`. Some lint issues require
manual changes. See the [golangci-lint command reference](https://golangci-lint.run/docs/configuration/cli/)
and [Go formatting guide](https://go.dev/blog/gofmt).

Before opening a pull request, run from the repository root:

```bash
golangci-lint run
go test ./...
```

## Open a pull request

Describe the change, link any related issue, and include the test results. If
the change affects setup or CLI behavior, update the relevant documentation.
