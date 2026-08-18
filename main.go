// Command gai — "git ai": AI helpers for git, powered by the Claude CLI
// (subscription auth, no API key).
//
// Usage:
//
//	gai <command> [options]
//	gai commit [-a -y -p -e]   generate a commit message and commit
//	gai help                   list commands
//	gai <command> -h           help for a command
//
// Adding a new command is self-contained: write a func of type CommandFunc in
// its own file and add one line to the registry in commands().
package main

import (
	"fmt"
	"os"
	"sort"
)

// CommandFunc runs a subcommand with its own args (everything after the verb).
type CommandFunc func(args []string) error

// Command is a registered verb.
type Command struct {
	Name    string
	Summary string      // one-line description for `gai help`
	Run     CommandFunc // does the work; handles its own -h
}

// commands is the single registry. To add a verb, add an entry here and write
// its Run func in a file (e.g. cmd_pr.go). Nothing else needs to change.
func commands() []Command {
	return []Command{
		{
			Name:    "commit",
			Summary: "Generate a Conventional-Commits message from the staged diff and commit",
			Run:     runCommit,
		},
		// Internal helpers the lazygit keybinding calls. Hidden from `help` via
		// the leading underscore convention handled in printHelp.
		{Name: "generate", Summary: "_run Claude on staged diff, cache, print summary line", Run: runGenerate},
		{Name: "body", Summary: "_print the cached body", Run: runBody},
		{Name: "full", Summary: "_print the cached full message", Run: runFull},

		// Future verbs slot in here, e.g.:
		// {Name: "pr", Summary: "Draft a pull-request description from the branch diff", Run: runPR},
		// {Name: "explain", Summary: "Explain a commit or diff in plain language", Run: runExplain},
		// {Name: "changelog", Summary: "Generate a changelog from a commit range", Run: runChangelog},
	}
}

func main() {
	if err := dispatch(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "gai:", err)
		os.Exit(1)
	}
}

func dispatch(args []string) error {
	if len(args) == 0 {
		printHelp(os.Stdout)
		return nil
	}
	verb := args[0]
	switch verb {
	case "-h", "--help", "help":
		printHelp(os.Stdout)
		return nil
	}
	for _, c := range commands() {
		if c.Name == verb {
			return c.Run(args[1:])
		}
	}
	fmt.Fprintf(os.Stderr, "gai: unknown command %q\n\n", verb)
	printHelp(os.Stderr)
	os.Exit(2)
	return nil
}

func printHelp(w *os.File) {
	fmt.Fprint(w, `gai — git ai: AI helpers for git, via the Claude CLI

USAGE
  gai <command> [options]

COMMANDS
`)
	cmds := commands()
	sort.Slice(cmds, func(i, j int) bool { return cmds[i].Name < cmds[j].Name })
	for _, c := range cmds {
		if len(c.Summary) > 0 && c.Summary[0] == '_' {
			continue // hidden internal helper
		}
		fmt.Fprintf(w, "  %-10s %s\n", c.Name, c.Summary)
	}
	fmt.Fprint(w, `
Run `+"`gai <command> -h`"+` for command-specific options.

CONFIG
  Prompts can be overridden per command via env vars or files under
  ~/.config/gai/ (see each command's help). Requires the `+"`claude`"+` CLI
  logged in via your subscription (check with `+"`claude /status`"+`).
`)
}
