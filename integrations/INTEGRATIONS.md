# Integrations

`verify integration name: telegram, github` at the prompt checks that the
credentials for a service actually work, without leaving the loop.

The wiring is complete and runnable. The two checks themselves are stubs: they
return `ErrNotImplemented`, and the CLI reports that as "nothing was checked"
rather than as a verdict.

## Shape

Registry plus strategy, the same arrangement the tool registry already uses.

- `integrations.IVerifier` — what one integration implements: its name, a line
  describing it, the `Field`s it needs, and `Verify`.
- `integrations.Registry` — name to verifier, so the dispatcher never holds a
  list of services.
- `cli/cli.verify.go` — parses the command, asks for each field on the loop's
  own stdin, prints the verdict.
- `cli/cli.integrations.go` — the list of available integrations, and the only
  file that names them.

A verifier never reads stdin and never prints. It is handed a map of answers and
returns a verdict, which is what makes it testable and what keeps the terminal
under the CLI's control. It also matters mechanically: two readers on one stdin
lose input to each other's buffers, so a verifier must not open its own.

## The outcomes

Kept apart on purpose, because they call for different reactions:

- `Result{OK: true}` — the service confirmed the credential.
- `Result{OK: false}` — the check ran and the credential was refused. The
  `Summary` says why.
- a non-nil `error` — the check could not be made at all: unreachable host,
  unreadable reply, an endpoint that is not the API it claims to be.
- `ErrNotImplemented` — the check does not exist yet. Distinct from the above so
  a stub can never read as a judgement on a token.

## Secrets

A `Field` marked `Secret` is not echoed while typed and never written to the
session transcript; `<key withheld>` is recorded in its place. Hiding the echo
depends on `stty`, so it degrades to visible typing rather than to a failed
check — the transcript rule is what actually holds.

## Filling in a stub

Replace the `Verify` body. Each package's doc comment on `Verify` records the
request to make and how to read the answer; the service's `.md` alongside it has
the reference links.

There is no shared HTTP helper yet. Once both checks are written and the
duplication is visible, that is the moment to add one.

## Adding an integration

1. New package under `integrations/`, one type implementing `IVerifier`.
2. Add it to the list in `cli/cli.integrations.go`.

Nothing else changes. Parsing, prompting, secret handling and reporting are all
written against the interface.
