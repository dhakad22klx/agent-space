# Install JustSay

Install the latest published release on Linux or macOS, with an x86_64
or ARM64 CPU (including Apple Silicon). WSL is supported through Linux; native
Windows and 32-bit CPUs are not supported by this installer.

You need Go, `curl`, `tar`, a POSIX shell, and
`sha256sum` (Linux) or `shasum` (macOS). Install Go from
[go.dev/dl](https://go.dev/dl/) if needed. You do not need a repository checkout,
Redis, or administrator privileges.

```bash
curl -fsSL https://raw.githubusercontent.com/dhakad22klx/justsay/main/install.sh | sh
justsay
```

Using `| bash` instead of `| sh` also works. The installer resolves GitHub's latest
release once and downloads its source archive. It logs `go version`, checks the
Go version required by that release's `go.mod`, and runs `go mod download`.
Dependencies are installed in your Go module cache.

If the release has a matching `justsay_linux_amd64`, `justsay_linux_arm64`,
`justsay_darwin_amd64`, or `justsay_darwin_arm64` binary asset, the installer uses
it and verifies its SHA-256 checksum when `checksums.txt` is published. Otherwise,
it builds the downloaded release source with Go. Source-only releases work, and
the installer never builds unreleased changes from `main`.

The executable is installed at `~/.local/bin/justsay`, with executable permissions
checked. Run the same command again to update to your latest published release.
A failed download, dependency installation, checksum verification, or build
leaves an existing binary intact.

## PATH

The installer adds an idempotent PATH block to your shell's startup files:

| Shell | Files |
| --- | --- |
| Bash | `~/.bashrc`, plus the first existing file of `~/.bash_profile`, `~/.bash_login`, or `~/.profile` (creating `~/.profile` if none exists) |
| Zsh | `${ZDOTDIR:-$HOME}/.zshrc` and `${ZDOTDIR:-$HOME}/.zprofile` |
| Fish | `${XDG_CONFIG_HOME:-$HOME/.config}/fish/conf.d/justsay.fish` |
| Sh, Dash, Ksh | `~/.profile` |

A piped script cannot change PATH in the shell that launched it. If
`~/.local/bin` is not already on PATH, open a new terminal or run:

```bash
export PATH="$HOME/.local/bin:$PATH"
justsay
```

For Fish, run `set -gx PATH "$HOME/.local/bin" $PATH` instead. Other shells
need their equivalent PATH setting in their startup file. You can always start
the CLI immediately with `"$HOME/.local/bin/justsay"`.

## Verify and start

```bash
command -v justsay
test -x "$HOME/.local/bin/justsay" && echo "JustSay is installed"
justsay
```

`command -v justsay` should print the path to `~/.local/bin/justsay`. Running
`justsay` with no arguments starts the interactive agent.

Model requests still need your Gemini credentials. In the directory where you
run `justsay`, create a `.env` file containing:

```dotenv
GEMINI_API_KEY="your Gemini API key"
GEMINI_MODEL="your Gemini model ID"
MOCK_AGENT_CALL="false"
```

See [.env.example](.env.example) for optional integrations and state settings.
The CLI opens without `.env`, but cannot answer prompts until the model is
configured. Credentials and session transcripts are stored in the working
directory, as described in [README.md](README.md).

## Troubleshooting

- **Command not found:** apply the PATH command above or use
  `"$HOME/.local/bin/justsay"`. `command -v justsay` should resolve to
  the installed binary.
- **Release or binary unavailable:** check the
  [releases page](https://github.com/dhakad22klx/justsay/releases). There must be a
  published release with Go source. A missing binary asset triggers a source
  build automatically. Publishing a new latest release makes it available to
  the same installer command; no installer update is needed.
- **Download failed:** check your network connection and access to GitHub, then
  rerun the installer.
- **Checksum mismatch:** rerun the installer. If it repeats, report the release
  tag shown in the install log.
- **Permission denied:** ensure you can write to `~/.local/bin` and your shell
  startup files, then rerun the installer without `sudo`.
- **Go missing or too old:** install the Go version printed by the installer
  (or newer), check `go version`, and rerun the installer.
- **Go dependencies failed:** check network access to your configured Go module
  proxy and write access to your Go module cache, then rerun the installer.

## Manual removal

```bash
rm -f "$HOME/.local/bin/justsay"
```

Optionally remove the block between `# >>> JustSay PATH >>>` and
`# <<< JustSay PATH <<<` from the startup files listed above. For Fish, you can
remove `justsay.fish`. Keep `~/.local/bin` on PATH if other tools use it.
Removing the binary leaves your `.env`, `credentials.json`, and `sessions/`
files in place; remove those separately only if you no longer need them.
