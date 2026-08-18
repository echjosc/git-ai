package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// --- claude -----------------------------------------------------------------

// runAI pipes the prompt to the configured AI CLI on stdin and returns stdout.
//
// The command is configurable so gai isn't locked to one provider:
//   - $GAI_AI_CMD    full command line, e.g. "claude -p --max-turns 1"
//     or "codex exec" or "ollama run llama3" or a wrapper script.
//     Takes precedence over everything below.
//   - $GAI_CLAUDE_BIN  swaps just the binary name in the default command
//     (still "-p --max-turns 1 [--model ...]"). Must be a real executable
//     on $PATH — shell aliases don't resolve here since exec.Command
//     bypasses the shell. For a "different claude login/config" alias,
//     use $GAI_CLAUDE_CONFIG_DIR instead (below).
//   - $GAI_CLAUDE_CONFIG_DIR  sets CLAUDE_CONFIG_DIR for the child process,
//     e.g. to switch between a personal and work claude login without
//     needing a shell alias. Applied regardless of backend; harmless if
//     the backend isn't claude.
//   - $GAI_MODEL     if set AND using the default claude command, appends
//     "--model <value>" (convenience for the common case).
//
// Default: claude -p --max-turns 1  (subscription auth, no API key).
//
// The prompt is always delivered on stdin, so any tool that reads stdin and
// writes the result to stdout works as a drop-in backend.
func runAI(stdin string) (string, error) {
	name, args := aiCommand()
	cmd := exec.Command(name, args...)
	cmd.Stdin = strings.NewReader(stdin)
	if dir := strings.TrimSpace(os.Getenv("GAI_CLAUDE_CONFIG_DIR")); dir != "" {
		cmd.Env = append(os.Environ(), "CLAUDE_CONFIG_DIR="+dir)
	}
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		if msg := strings.TrimSpace(errb.String()); msg != "" {
			return "", fmt.Errorf("%s: %v: %s", name, err, msg)
		}
		return "", fmt.Errorf("%s: %v", name, err)
	}
	return out.String(), nil
}

// aiCommand resolves the backend command line into a binary + args.
func aiCommand() (string, []string) {
	if custom := strings.TrimSpace(os.Getenv("GAI_AI_CMD")); custom != "" {
		fields := strings.Fields(custom)
		return fields[0], fields[1:]
	}
	// default: claude (or $GAI_CLAUDE_BIN, e.g. a work-account binary),
	// with optional model override
	bin := "claude"
	if b := strings.TrimSpace(os.Getenv("GAI_CLAUDE_BIN")); b != "" {
		bin = b
	}
	args := []string{"-p", "--max-turns", "1"}
	if m := strings.TrimSpace(os.Getenv("GAI_MODEL")); m != "" {
		args = append(args, "--model", m)
	}
	return bin, args
}

// backendLabel returns a human-friendly name for the configured AI backend,
// used in spinner/status text.
func backendLabel() string {
	name, _ := aiCommand()
	switch {
	case name == "claude" || strings.HasPrefix(name, "claude"):
		return "Claude"
	case name == "codex":
		return "Codex"
	case name == "ollama":
		return "Ollama"
	default:
		return name
	}
}

// --- git --------------------------------------------------------------------

func ensureRepo() error {
	if _, err := gitOut("rev-parse", "--is-inside-work-tree"); err != nil {
		return errors.New("not a git repository")
	}
	return nil
}

func hasStaged() (bool, error) {
	// `git diff --cached --quiet` exits 1 when there ARE staged changes.
	err := exec.Command("git", "diff", "--cached", "--quiet").Run()
	if err == nil {
		return false, nil
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) && ee.ExitCode() == 1 {
		return true, nil
	}
	return false, err
}

func gitRun(args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func gitOut(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return out.String(), nil
}

func commitMessage(msg string) error {
	cmd := exec.Command("git", "commit", "-F", "-")
	cmd.Stdin = strings.NewReader(ensureTrailingNewline(msg))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// --- editor -----------------------------------------------------------------

func editInEditor(msg string) (string, error) {
	ed := os.Getenv("EDITOR")
	if ed == "" {
		ed = "vi"
	}
	tmp, err := os.CreateTemp("", "gai-*.txt")
	if err != nil {
		return msg, err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err := tmp.WriteString(ensureTrailingNewline(msg)); err != nil {
		tmp.Close()
		return msg, err
	}
	tmp.Close()

	cmd := exec.Command(ed, name)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return msg, err
	}
	b, err := os.ReadFile(name)
	if err != nil {
		return msg, err
	}
	return strings.TrimRight(string(b), "\n"), nil
}

// --- message parsing (pure) -------------------------------------------------

func cleanMessage(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	lines := strings.Split(s, "\n")
	var out []string
	for _, ln := range lines {
		t := strings.TrimSpace(ln)
		if t == "```" || (strings.HasPrefix(t, "```") && !strings.Contains(t, " ")) {
			continue
		}
		out = append(out, ln)
	}
	for len(out) > 0 && strings.TrimSpace(out[0]) == "" {
		out = out[1:]
	}
	for len(out) > 0 && strings.TrimSpace(out[len(out)-1]) == "" {
		out = out[:len(out)-1]
	}
	return strings.Join(out, "\n")
}

func summaryOf(full string) string {
	full = strings.ReplaceAll(full, "\r\n", "\n")
	if i := strings.IndexByte(full, '\n'); i >= 0 {
		return full[:i]
	}
	return full
}

func bodyOf(full string) string {
	full = strings.ReplaceAll(full, "\r\n", "\n")
	i := strings.IndexByte(full, '\n')
	if i < 0 {
		return ""
	}
	rest := full[i+1:]
	if len(rest) > 0 && rest[0] == '\n' {
		rest = rest[1:]
	}
	return strings.TrimRight(rest, "\n")
}

func ensureTrailingNewline(s string) string {
	if strings.HasSuffix(s, "\n") {
		return s
	}
	return s + "\n"
}

// --- cache (per-repo, in .git) ---------------------------------------------

func cachePath() (string, error) {
	dir, err := gitOut("rev-parse", "--git-dir")
	if err != nil {
		return "", err
	}
	return filepath.Join(strings.TrimSpace(dir), ".gai-cache"), nil
}

func writeCache(msg string) error {
	p, err := cachePath()
	if err != nil {
		return err
	}
	return os.WriteFile(p, []byte(msg), 0o644)
}

func readCache() (string, error) {
	p, err := cachePath()
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// --- prompt config (per-command) -------------------------------------------

// loadPrompt returns a prompt for a command, preferring: the env var, then
// ~/.config/gai/<name>.txt, then the built-in fallback.
func loadPrompt(name, envVar, fallback string) string {
	if v := os.Getenv(envVar); strings.TrimSpace(v) != "" {
		return v
	}
	if p := promptFilePath(name); p != "" {
		if b, err := os.ReadFile(p); err == nil && strings.TrimSpace(string(b)) != "" {
			return string(b)
		}
	}
	return fallback
}

func promptFilePath(name string) string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "gai", name+".txt")
}

// --- tty --------------------------------------------------------------------

func readTTYLine() (string, error) {
	tty, err := os.Open("/dev/tty")
	if err != nil {
		var s string
		_, e := fmt.Scanln(&s)
		return s, e
	}
	defer tty.Close()
	var buf []byte
	one := make([]byte, 1)
	for {
		n, err := tty.Read(one)
		if n > 0 {
			if one[0] == '\n' {
				break
			}
			buf = append(buf, one[0])
		}
		if err != nil {
			break
		}
	}
	return string(buf), nil
}
