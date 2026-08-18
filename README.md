# Git AI - gai

A single Go binary of AI helpers for git, powered by the Claude CLI
(subscription auth, no API key). Built as a verb dispatcher so it can grow:
`gai commit` today, `gai pr` / `gai explain` / `gai changelog` later, each a
self-contained command.

## Install

Download a prebuilt binary from the [latest release](https://github.com/echjosc/git-ai/releases/latest)
(macOS/Linux/Windows, amd64/arm64), extract it, and put `gai` on your `PATH`:

```bash
curl -sSL https://github.com/echjosc/git-ai/releases/latest/download/gai_darwin_arm64.tar.gz | tar xz
mv gai ~/.local/bin/gai
```

(swap `darwin_arm64` for your OS/arch, e.g. `linux_amd64`)

Or build from source:

```bash
cd git-ai
go build -o gai .
mv gai ~/.local/bin/gai        # anywhere on your PATH
# or:
go install .
```

Requires Go 1.26+ and the `claude` CLI logged in via your subscription
(`claude /status` should show subscription auth, not an API key).

## Usage

```
gai                    list commands
gai commit             generate a commit message for the staged diff, review, commit
gai commit -a          stage all tracked changes first
gai commit -y          no prompt, commit immediately
gai commit -p          push after commit
gai commit -e          edit in $EDITOR before committing
gai commit -ayp        stage all, no prompt, commit, push
gai commit -h          command help
```

Grouped workflow:

```bash
git add -p && gai commit    # stage a logical group, review, commit
# ...repeat...
git push                    # one push at the end
```

## Tweaking prompts (no recompile)

Each command reads its prompt from an env var or a file, falling back to a
built-in default:

- `gai commit` → `$GAI_COMMIT_PROMPT` or `~/.config/gai/commit.txt`

Future commands follow the same pattern (`~/.config/gai/<command>.txt`).

## Choosing the AI backend (no recompile)

gai pipes the prompt to a CLI on stdin and reads stdout, so the backend is
swappable:

- `$GAI_MODEL` — with the default claude backend, picks the model, e.g.
  `export GAI_MODEL=claude-opus-4-8`.
- `$GAI_CLAUDE_CONFIG_DIR` — sets `CLAUDE_CONFIG_DIR` for the child process,
  e.g. to switch between a personal and work `claude` login without a shell
  alias: `export GAI_CLAUDE_CONFIG_DIR=$HOME/.config/claude-work`. Applied
  regardless of backend; harmless if you're not using claude.
- `$GAI_CLAUDE_BIN` — swaps just the binary name in the default command
  (still `-p --max-turns 1 [--model ...]`). Must be a real executable on
  `$PATH` — **shell aliases won't work here**, since gai execs the binary
  directly and never goes through your shell, so alias definitions are
  invisible to it. If you have a shell alias that just sets
  `CLAUDE_CONFIG_DIR`, prefer `$GAI_CLAUDE_CONFIG_DIR` above instead.
- `$GAI_AI_CMD` — replace the whole command. Any tool that reads stdin and
  writes the result to stdout works:
    - `export GAI_AI_CMD="claude -p --max-turns 1"`  (the default)
    - `export GAI_AI_CMD="codex exec"`
    - `export GAI_AI_CMD="ollama run llama3"`         (fully local; nothing leaves
      the machine — good for sensitive repos)
    - `export GAI_AI_CMD="/path/to/my-wrapper.sh"`

`$GAI_AI_CMD` takes precedence over `$GAI_CLAUDE_BIN`/`$GAI_MODEL`.
`$GAI_CLAUDE_CONFIG_DIR` applies either way. Default (nothing set):
`claude -p --max-turns 1`, using your Claude subscription (no API key).

## lazygit integration

```yaml
customCommands:
  - key: "X"
    context: "files"
    description: "AI commit (one shot)"
    loadingText: "Claude is writing the commit..."
    command: "gai commit -y"
```

`gai commit -y` generates and commits with no prompt. After it runs, press `4`
to hit the Commits panel — your new commit is on top, selected, diff on the
right. Keep the panel fresh:

```yaml
git:
  autoRefresh: true
  autoDetectExternalChanges: true
```

(The internal `gai generate` / `gai body` / `gai full` verbs also exist if you
ever build a multi-box prompt flow.)

## Adding a new command

The registry in `main.go` (`commands()`) is the only place the dispatcher looks.
To add `gai pr`:

1. Write `runPR(args []string) error` in a new `cmd_pr.go`.
2. Add one line to `commands()`:
   `{Name: "pr", Summary: "Draft a PR description", Run: runPR}`.

Shared helpers (git exec, claude exec, cache, prompt loading, message parsing)
live in `helpers.go` and are reusable by any command. Nothing else changes.

## Layout

```
main.go         dispatcher + command registry + top-level help
cmd_commit.go   the `commit` command (flags, prompt, generate+commit, helper verbs)
helpers.go      shared: git, claude, cache, prompt config, tty, message parsing
main_test.go    tests for message parsing and flag logic
```

## Tests

```bash
go test ./...
```