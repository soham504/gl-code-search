// Command gl is a code-search tool for bug localization, usable from the
// shell, from CI, or by coding agents over MCP. It indexes source files under
// -root (default: current directory); an on-disk analysis cache keyed by file
// size and mtime makes repeat calls cheap and keeps results fresh as files change.
//
//	gl locate < issue.txt      rank files likely to need changes for an issue
//	gl search QUERY...         ranked files with their best-matching lines
//	gl symbol NAME             where NAME is defined
//	gl mcp                     serve the three tools above over MCP (stdio)
//	gl hook                    Claude Code UserPromptSubmit hook: inject a shortlist when confident
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/soham504/gl-code-search/go/internal/code"
)

const version = "0.2.0"

// load indexes root, reusing and then updating the analysis cache.
func load(root string, useCache bool) (*code.Index, time.Duration, error) {
	start := time.Now()
	files, err := code.LoadDir(root)
	if err != nil {
		return nil, 0, err
	}
	var c *code.Cache
	if useCache {
		c = code.OpenCache(root)
	}
	ix := code.BuildCached(files, c)
	if err := c.Save(); err != nil {
		fmt.Fprintln(os.Stderr, "gl: cache not saved:", err)
	}
	return ix, time.Since(start).Round(time.Millisecond), nil
}

func locateText(ix *code.Index, issue string, k int, took time.Duration) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %d files indexed in %v; most likely files for this issue:\n", ix.Len(), took)
	for n, r := range ix.Locate(issue, k, code.Options{}) {
		fmt.Fprintf(&b, "%2d. %s  (%.2f; %s)\n", n+1, r.Path, r.Score, strings.Join(r.Reasons, "; "))
	}
	return b.String()
}

// locateMarkdown formats results for a GitHub issue comment.
func locateMarkdown(ix *code.Index, issue string, k int) string {
	var b strings.Builder
	b.WriteString("**Likely files for this issue** (ranked by [gl](https://github.com/soham504/gl-code-search), no LLM involved):\n\n")
	for n, r := range ix.Locate(issue, k, code.Options{}) {
		fmt.Fprintf(&b, "%d. `%s` — %s\n", n+1, r.Path, strings.Join(r.Reasons, "; "))
	}
	b.WriteString("\n<sub>Automated suggestion. On held-out benchmarks the fixed file is in this top-5 about 70% of the time.</sub>\n")
	return b.String()
}

func searchText(ix *code.Index, query string, k int) string {
	var b strings.Builder
	for n, r := range ix.Locate(query, k, code.Options{NoTrace: true}) {
		fmt.Fprintf(&b, "%2d. %s\n", n+1, r.Path)
		for _, s := range ix.Snippet(r.Path, query, 3) {
			fmt.Fprintf(&b, "      %s\n", s)
		}
	}
	if b.Len() == 0 {
		return "no matches\n"
	}
	return b.String()
}

func symbolText(ix *code.Index, name string) string {
	defs := ix.Definitions(strings.TrimSpace(name))
	if len(defs) == 0 {
		return "no definition found\n"
	}
	return strings.Join(defs, "\n") + "\n"
}

func main() {
	root := flag.String("root", ".", "source tree to index")
	k := flag.Int("k", 10, "results to show")
	asJSON := flag.Bool("json", false, "locate: print results as JSON")
	asMarkdown := flag.Bool("markdown", false, "locate: print an issue-comment-ready Markdown list")
	gate := flag.Bool("gate", false, "locate: print nothing unless the ranking is confident (for bots)")
	noCache := flag.Bool("no-cache", false, "do not read or write the analysis cache")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: gl [-root DIR] [-k N] [-json] [-no-cache] locate < issue.txt | search QUERY... | symbol NAME | mcp | hook | version")
	}
	flag.Parse()
	if flag.NArg() == 0 {
		flag.Usage()
		os.Exit(2)
	}
	cmd, rest := flag.Arg(0), strings.Join(flag.Args()[1:], " ")
	switch cmd {
	case "version":
		fmt.Println("gl", version)
		return
	case "hook":
		runHook(os.Stdin, os.Stdout, *root, *k)
		return
	case "mcp":
		if err := serveMCP(os.Stdin, os.Stdout, *root, !*noCache); err != nil {
			fmt.Fprintln(os.Stderr, "gl mcp:", err)
			os.Exit(1)
		}
		return
	}

	ix, took, err := load(*root, !*noCache)
	if err != nil {
		fmt.Fprintln(os.Stderr, "gl:", err)
		os.Exit(1)
	}
	switch cmd {
	case "locate":
		b, _ := io.ReadAll(os.Stdin)
		issue := string(b)
		if strings.TrimSpace(issue) == "" {
			issue = rest
		}
		if *gate && !code.Confident(ix.Locate(issue, *k, code.Options{})) {
			return
		}
		if *asJSON {
			json.NewEncoder(os.Stdout).Encode(ix.Locate(issue, *k, code.Options{}))
			return
		}
		if *asMarkdown {
			fmt.Print(locateMarkdown(ix, issue, *k))
			return
		}
		fmt.Print(locateText(ix, issue, *k, took))
	case "search":
		if rest == "" {
			flag.Usage()
			os.Exit(2)
		}
		fmt.Print(searchText(ix, rest, *k))
	case "symbol":
		fmt.Print(symbolText(ix, rest))
	default:
		flag.Usage()
		os.Exit(2)
	}
}
