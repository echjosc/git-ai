package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

const commitDefaultPrompt = `Analyze these staged changes and write a detailed git commit message in
Conventional Commits format. Begin with a concise imperative summary line
(<=72 chars total including the type/scope prefix, no trailing period) prefixed
with an appropriate type and scope, e.g. "feat(auth): ..." or "fix(database): ...".
Then a blank line, then a bulleted list (use "- ") of 3-5 concise points
explaining the key technical details and the WHY behind the changes. Output ONLY
the raw commit message text: no markdown wrapper, no triple backticks, no chat,
no questions.`

type commitFlags struct {
	stageAll bool
	yes      bool
	push     bool
	edit     bool
}

func parseCommitFlags(args []string) (commitFlags, error) {
	var f commitFlags
	for _, a := range args {
		switch a {
		case "-h", "--help":
			commitUsage(os.Stdout)
			os.Exit(0)
		case "--":
			continue
		}
		if len(a) >= 2 && a[0] == '-' && a[1] != '-' {
			for _, c := range a[1:] {
				switch c {
				case 'a':
					f.stageAll = true
				case 'y':
					f.yes = true
				case 'p':
					f.push = true
				case 'e':
					f.edit = true
				default:
					return f, fmt.Errorf("unknown option -%c", c)
				}
			}
		} else {
			return f, fmt.Errorf("unexpected argument %q", a)
		}
	}
	return f, nil
}

// runCommit is the interactive/CLI path.
func runCommit(args []string) error {
	f, err := parseCommitFlags(args)
	if err != nil {
		commitUsage(os.Stderr)
		os.Exit(2)
	}
	if err := ensureRepo(); err != nil {
		return err
	}
	if f.stageAll {
		if err := gitRun("add", "-A"); err != nil {
			return err
		}
	}
	staged, err := hasStaged()
	if err != nil {
		return err
	}
	if !staged {
		return errors.New("nothing staged (use -a to stage all tracked changes)")
	}

	msg, err := generateCommit()
	if err != nil {
		return err
	}
	if f.edit {
		msg, err = editInEditor(msg)
		if err != nil {
			return err
		}
	}
	if !f.yes {
		line := strings.Repeat("-", 64)
		fmt.Println(line)
		fmt.Println(msg)
		fmt.Println(line)
		fmt.Print("Commit? [y]es / [e]dit / [n]o: ")
		ans, _ := readTTYLine()
		switch strings.ToLower(strings.TrimSpace(ans)) {
		case "y", "yes":
		case "e", "edit":
			msg, err = editInEditor(msg)
			if err != nil {
				return err
			}
		default:
			fmt.Println("aborted.")
			return nil
		}
	}
	if err := commitMessage(msg); err != nil {
		return err
	}
	if f.push {
		return gitRun("push")
	}
	return nil
}

// generateCommit runs Claude on the staged diff, cleans + caches the message.
func generateCommit() (string, error) {
	diff, err := gitOut("diff", "--cached")
	if err != nil {
		return "", err
	}
	prompt := loadPrompt("commit", "GAI_COMMIT_PROMPT", commitDefaultPrompt)
	input := prompt + "\n\n=== STAGED DIFF ===\n" + diff
	out, err := withSpinner(backendLabel()+" is writing the commit message...", func() (string, error) {
		return runAI(input)
	})
	if err != nil {
		return "", fmt.Errorf("AI backend failed: %v", err)
	}
	msg := cleanMessage(out)
	if strings.TrimSpace(msg) == "" {
		msg = "chore: update"
	}
	if err := writeCache(msg); err != nil {
		return "", err
	}
	return msg, nil
}

// --- lazygit helper verbs ---------------------------------------------------

func runGenerate(_ []string) error {
	if err := ensureRepo(); err != nil {
		return err
	}
	staged, err := hasStaged()
	if err != nil {
		return err
	}
	if !staged {
		fmt.Println("(nothing staged)")
		_ = writeCache("")
		return nil
	}
	msg, err := generateCommit()
	if err != nil {
		return err
	}
	fmt.Println(summaryOf(msg))
	return nil
}

func runBody(_ []string) error {
	full, err := readCache()
	if err != nil {
		fmt.Println("")
		return nil
	}
	fmt.Println(bodyOf(full))
	return nil
}

func runFull(_ []string) error {
	full, err := readCache()
	if err != nil {
		return errors.New("no cached message; run `gai generate` first")
	}
	fmt.Print(ensureTrailingNewline(full))
	return nil
}

func commitUsage(w *os.File) {
	fmt.Fprint(w, `gai commit — generate a commit message and commit

USAGE
  gai commit [options]

OPTIONS
  -a          Stage all tracked changes (git add -A) first
  -y          Skip the confirmation prompt; commit immediately
  -p          Push after a successful commit
  -e          Open the message in $EDITOR before committing
  -h, --help  Show this help

Flags combine, e.g.  gai commit -ayp

CONFIG
  Override the prompt with $GAI_COMMIT_PROMPT or ~/.config/gai/commit.txt.

EXAMPLES
  git add -p && gai commit     Stage hunks, review, commit
  gai commit -ay               Stage all and commit without prompting
  gai commit -p                Commit staged diff, then push
`)
}
