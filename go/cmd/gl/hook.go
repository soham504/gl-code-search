package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/soham504/gl-code-search/go/internal/code"
)

// runHook implements a Claude Code UserPromptSubmit hook. It reads the hook
// event from stdin and, when gl is confident about the prompt, adds the top
// files as context for the model, so the agent starts with a shortlist
// instead of having to choose to call a tool. It never blocks the prompt:
// on any problem it prints nothing and exits 0.
func runHook(in io.Reader, out io.Writer, root string, k int) {
	var ev struct {
		Prompt string `json:"prompt"`
		Cwd    string `json:"cwd"`
	}
	if err := json.NewDecoder(in).Decode(&ev); err != nil || len(strings.Fields(ev.Prompt)) < 8 {
		return
	}
	if root == "." && ev.Cwd != "" {
		root = ev.Cwd
	}
	ix, _, err := load(root, true)
	if err != nil {
		return
	}
	rs := ix.Locate(ev.Prompt, k, code.Options{})
	if !code.Confident(rs) {
		return
	}
	json.NewEncoder(out).Encode(map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName":     "UserPromptSubmit",
			"additionalContext": contextText(rs),
		},
	})
}

// contextText is the shortlist an agent sees, shared by the hook and the
// agent experiment so both inject identical text.
func contextText(rs []code.Result) string {
	var b strings.Builder
	b.WriteString("gl (deterministic code search, no LLM) ranked these files as most likely relevant to this request. " +
		"It is a starting point, not ground truth; verify before editing.\n")
	for n, r := range rs {
		fmt.Fprintf(&b, "%d. %s (%s)\n", n+1, r.Path, strings.Join(r.Reasons, "; "))
	}
	return b.String()
}
