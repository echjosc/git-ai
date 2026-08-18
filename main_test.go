package main

import (
	"os"
	"testing"
)

func TestCleanMessage(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{
			name: "plain passthrough",
			in:   "feat(x): do a thing\n\n- because reasons",
			want: "feat(x): do a thing\n\n- because reasons",
		},
		{
			name: "strip bare fences",
			in:   "```\nfix(y): patch\n\n- detail\n```",
			want: "fix(y): patch\n\n- detail",
		},
		{
			name: "strip language fence",
			in:   "```text\nchore: bump\n```",
			want: "chore: bump",
		},
		{
			name: "crlf normalized",
			in:   "feat: a\r\n\r\n- b\r\n",
			want: "feat: a\n\n- b",
		},
		{
			name: "leading and trailing blanks dropped",
			in:   "\n\nfeat: a\n\n- b\n\n\n",
			want: "feat: a\n\n- b",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := cleanMessage(c.in); got != c.want {
				t.Errorf("cleanMessage(%q)\n got: %q\nwant: %q", c.in, got, c.want)
			}
		})
	}
}

func TestSummaryOf(t *testing.T) {
	if got := summaryOf("feat: a\n\n- b"); got != "feat: a" {
		t.Errorf("summary = %q", got)
	}
	if got := summaryOf("only one line"); got != "only one line" {
		t.Errorf("single-line summary = %q", got)
	}
}

func TestBodyOf(t *testing.T) {
	cases := []struct{ in, want string }{
		{"feat: a\n\n- b\n- c", "- b\n- c"},
		{"feat: a\n- b", "- b"}, // no blank line between
		{"only summary", ""},    // no body
		{"feat: a\n\n", ""},     // empty body
	}
	for _, c := range cases {
		if got := bodyOf(c.in); got != c.want {
			t.Errorf("bodyOf(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestSummaryBodyRoundTrip(t *testing.T) {
	full := "feat(auth): add bearer token middleware\n\n- reject requests without a token\n- return 401 when absent"
	if summaryOf(full) != "feat(auth): add bearer token middleware" {
		t.Fatal("summary mismatch")
	}
	if bodyOf(full) != "- reject requests without a token\n- return 401 when absent" {
		t.Fatalf("body mismatch: %q", bodyOf(full))
	}
}

func TestParseFlags(t *testing.T) {
	cases := []struct {
		args    []string
		want    commitFlags
		wantErr bool
	}{
		{[]string{}, commitFlags{}, false},
		{[]string{"-a"}, commitFlags{stageAll: true}, false},
		{[]string{"-ayp"}, commitFlags{stageAll: true, yes: true, push: true}, false},
		{[]string{"-a", "-y", "-p", "-e"}, commitFlags{stageAll: true, yes: true, push: true, edit: true}, false},
		{[]string{"-z"}, commitFlags{}, true},
		{[]string{"bogus"}, commitFlags{}, true},
	}
	for _, c := range cases {
		got, err := parseCommitFlags(c.args)
		if c.wantErr {
			if err == nil {
				t.Errorf("parseCommitFlags(%v): expected error", c.args)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseCommitFlags(%v): unexpected error %v", c.args, err)
			continue
		}
		if got != c.want {
			t.Errorf("parseCommitFlags(%v) = %+v, want %+v", c.args, got, c.want)
		}
	}
}

func TestEnsureTrailingNewline(t *testing.T) {
	if ensureTrailingNewline("x") != "x\n" {
		t.Error("should add newline")
	}
	if ensureTrailingNewline("x\n") != "x\n" {
		t.Error("should not double newline")
	}
}

func TestAICommand(t *testing.T) {
	// Save and restore env.
	for _, k := range []string{"GAI_AI_CMD", "GAI_MODEL"} {
		old, ok := os.LookupEnv(k)
		defer func(k, v string, ok bool) {
			if ok {
				os.Setenv(k, v)
			} else {
				os.Unsetenv(k)
			}
		}(k, old, ok)
		os.Unsetenv(k)
	}

	eq := func(a, b []string) bool {
		if len(a) != len(b) {
			return false
		}
		for i := range a {
			if a[i] != b[i] {
				return false
			}
		}
		return true
	}

	os.Unsetenv("GAI_AI_CMD")
	os.Unsetenv("GAI_MODEL")
	if n, a := aiCommand(); n != "claude" || !eq(a, []string{"-p", "--max-turns", "1"}) {
		t.Errorf("default = %q %v", n, a)
	}

	os.Setenv("GAI_MODEL", "claude-opus-4-8")
	if n, a := aiCommand(); n != "claude" || !eq(a, []string{"-p", "--max-turns", "1", "--model", "claude-opus-4-8"}) {
		t.Errorf("with model = %q %v", n, a)
	}
	os.Unsetenv("GAI_MODEL")

	os.Setenv("GAI_AI_CMD", "ollama run llama3")
	if n, a := aiCommand(); n != "ollama" || !eq(a, []string{"run", "llama3"}) {
		t.Errorf("custom = %q %v", n, a)
	}
	// custom command wins over model
	os.Setenv("GAI_MODEL", "ignored")
	if n, a := aiCommand(); n != "ollama" || !eq(a, []string{"run", "llama3"}) {
		t.Errorf("custom should override model = %q %v", n, a)
	}
	os.Unsetenv("GAI_AI_CMD")
	os.Unsetenv("GAI_MODEL")
}
