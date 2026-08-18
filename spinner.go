package main

import (
	"fmt"
	"os"
	"time"
)

var spinnerFrames = [...]string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// withSpinner runs fn while animating a spinner + elapsed timer on stderr,
// e.g. "⠙ Claude is writing the commit message... (3s)".
//
// The animation is skipped when stderr isn't a terminal, so captured/piped
// output (lazygit's customCommands, `gai generate` inside a script) stays
// clean — lazygit already renders its own `loadingText` popup in that case.
func withSpinner(label string, fn func() (string, error)) (string, error) {
	if !isTerminal(os.Stderr) {
		return fn()
	}

	type result struct {
		out string
		err error
	}
	resultCh := make(chan result, 1)
	go func() {
		out, err := fn()
		resultCh <- result{out, err}
	}()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	start := time.Now()
	frame := 0
	for {
		select {
		case r := <-resultCh:
			fmt.Fprint(os.Stderr, "\r\033[K")
			return r.out, r.err
		case <-ticker.C:
			elapsed := time.Since(start).Round(time.Second)
			fmt.Fprintf(os.Stderr, "\r\033[K%s %s (%s)", spinnerFrames[frame%len(spinnerFrames)], label, elapsed)
			frame++
		}
	}
}

func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}
