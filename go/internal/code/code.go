// Package code builds a code-aware BM25 index over a source tree and ranks
// files for an issue report. It is the retrieval core of the `gl` agent tool.
package code

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/kljensen/snowball/english"

	"github.com/soham504/gl-code-search/go/internal/index"
)

// File is one source file in the tree. Size and ModTime are set by LoadDir
// and key the analysis cache; Symbols is filled in by Build when nil.
type File struct {
	Path    string
	Content string
	Symbols []Symbol
	Size    int64
	ModTime int64
}

// Symbol is a function, class or type definition.
type Symbol struct {
	Name string
	Kind string // def, class, function, func, fn, struct, interface, enum, trait, impl, module, type
	Line int
}

// defRe is a language-agnostic declaration matcher covering Python, JS/TS,
// Go, Rust, Ruby, PHP, Java/C#/Kotlin/Scala type declarations. It trades
// precision for breadth: it misses e.g. Java methods, which have no keyword.
var defRe = regexp.MustCompile(`(?m)^[ \t]*(?:(?:export|default|pub(?:\([a-z]+\))?|public|private|protected|` +
	`static|final|abstract|async|override|internal|open|sealed|data|unsafe|const)[ \t]+)*` +
	`(def|class|function|func|fn|struct|interface|enum|trait|impl|module|type|object)[ \t]+` +
	`(?:\([^)]*\)[ \t]*)?\*?([A-Za-z_$][A-Za-z0-9_$]*)`)

func parseSymbols(src string) []Symbol {
	var out []Symbol
	for _, m := range defRe.FindAllStringSubmatchIndex(src, -1) {
		line := 1 + strings.Count(src[:m[0]], "\n")
		out = append(out, Symbol{Kind: src[m[2]:m[3]], Name: src[m[4]:m[5]], Line: line})
	}
	return out
}

var sourceExt = map[string]bool{
	".py": true, ".js": true, ".jsx": true, ".mjs": true, ".cjs": true, ".ts": true, ".tsx": true,
	".java": true, ".go": true, ".rs": true, ".rb": true, ".php": true, ".c": true, ".h": true,
	".cc": true, ".cpp": true, ".hpp": true, ".kt": true, ".scala": true, ".cs": true,
}

// Indexable reports whether path is a source file worth indexing.
func Indexable(path string) bool {
	return sourceExt[filepath.Ext(path)] && !strings.Contains(path, "/.") && !strings.HasPrefix(path, ".") &&
		!strings.Contains(path, "node_modules/") && !strings.Contains(path, "vendor/") &&
		!strings.HasSuffix(path, ".min.js") && !strings.HasSuffix(path, ".d.ts")
}

var testRe = regexp.MustCompile(`(^|/)(tests?|testing|__tests__|spec|specs|testdata|fixtures?)/|(^|/)test_[^/]*$|` +
	`_test\.[a-z]+$|_spec\.rb$|\.(test|spec)\.[a-z]+$|Tests?\.(java|cs|kt|scala)$|(^|/)conftest\.py$`)

// IsTest reports whether path looks like test code. Gold patches in these
// benchmarks exclude tests, and so does an engineer looking for the bug.
func IsTest(path string) bool { return testRe.MatchString(path) }

// LoadDir reads indexable files under root.
func LoadDir(root string) ([]File, error) {
	var files []File
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if n := d.Name(); n != "." && (strings.HasPrefix(n, ".") || n == "node_modules" || n == "graphify-out") {
				return filepath.SkipDir
			}
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		if !Indexable(rel) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		files = append(files, File{Path: rel, Content: string(b), Size: info.Size(), ModTime: info.ModTime().UnixNano()})
		return nil
	})
	return files, err
}

// LoadGit reads indexable files at commit sha from a (bare) git repository
// without checking anything out, via one `git cat-file --batch` stream.
func LoadGit(gitDir, sha string) ([]File, error) {
	ls, err := exec.Command("git", "--git-dir", gitDir, "ls-tree", "-r", sha).Output()
	if err != nil {
		return nil, fmt.Errorf("ls-tree %s: %w", sha, err)
	}
	var paths, objs []string
	for _, line := range strings.Split(strings.TrimSpace(string(ls)), "\n") {
		// "<mode> blob <oid>\t<path>"
		tab := strings.IndexByte(line, '\t')
		f := strings.Fields(line[:tab])
		if len(f) != 3 || f[1] != "blob" || !Indexable(line[tab+1:]) {
			continue
		}
		paths = append(paths, line[tab+1:])
		objs = append(objs, f[2])
	}

	cmd := exec.Command("git", "--git-dir", gitDir, "cat-file", "--batch")
	cmd.Stdin = strings.NewReader(strings.Join(objs, "\n") + "\n")
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	r := bufio.NewReaderSize(out, 1<<20)
	files := make([]File, 0, len(paths))
	for _, p := range paths {
		hdr, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		var oid, typ string
		var size int
		if _, err := fmt.Sscanf(hdr, "%s %s %d", &oid, &typ, &size); err != nil {
			return nil, fmt.Errorf("cat-file header %q: %w", hdr, err)
		}
		buf := make([]byte, size+1) // content + trailing newline
		if _, err := io.ReadFull(r, buf); err != nil {
			return nil, err
		}
		files = append(files, File{Path: p, Content: string(buf[:size])})
	}
	return files, cmd.Wait()
}

var pyNoise = map[string]bool{
	"self": true, "def": true, "return": true, "import": true, "from": true, "none": true, "true": true,
	"false": true, "class": true, "elif": true, "else": true, "pass": true, "cls": true, "arg": true,
	"args": true, "kwarg": true, "kwargs": true, "py": true, "lambda": true,
}

// splitIdent splits camelCase and digit boundaries: "QuerySet2Json" -> query set 2 json.
func splitIdent(s string) []string {
	var parts []string
	rs := []rune(s)
	start := 0
	for i := 1; i < len(rs); i++ {
		prev, cur := rs[i-1], rs[i]
		next := rune(0)
		if i+1 < len(rs) {
			next = rs[i+1]
		}
		boundary := (unicode.IsLower(prev) && unicode.IsUpper(cur)) ||
			(unicode.IsUpper(prev) && unicode.IsUpper(cur) && unicode.IsLower(next)) ||
			(unicode.IsDigit(prev) != unicode.IsDigit(cur))
		if boundary {
			parts = append(parts, string(rs[start:i]))
			start = i
		}
	}
	return append(parts, string(rs[start:]))
}

// Analyze is the code-aware analyzer: each identifier contributes its whole
// lowercased form plus its stemmed camelCase/snake_case parts, so the issue
// phrase "saving a date field" matches `save_date_field` and `DateField`.
func Analyze(s string) []string {
	var out []string
	for _, word := range strings.FieldsFunc(s, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' }) {
		lw := strings.ToLower(strings.Trim(word, "_"))
		if lw == "" {
			continue
		}
		var parts []string
		for _, snake := range strings.Split(word, "_") {
			if snake != "" {
				parts = append(parts, splitIdent(snake)...)
			}
		}
		if len(parts) > 1 && !pyNoise[lw] {
			out = append(out, lw)
		}
		for _, p := range parts {
			p = strings.ToLower(p)
			if len(p) < 2 || pyNoise[p] || stop[p] {
				continue
			}
			out = append(out, english.Stem(p, false))
		}
	}
	return out
}

var stop = map[string]bool{}

func init() {
	for _, w := range strings.Fields("a an and are as at be but by for if in into is it no not of on or such that the their then there these they this to was will with i we you my our can should would when what which how do does using use get set") {
		stop[w] = true
	}
}

// Index ranks files for free-text or issue queries.
type Index struct {
	bm25    *index.Index
	files   []File
	byPath  map[string]int
	symbols map[string][]int // lowercased symbol name -> file numbers
}

// Build indexes files. The "title" field (counted twice by index.Index) holds
// the path and defined symbol names, a cheap field boost for both.
func Build(files []File) *Index { return BuildCached(files, nil) }

// BuildCached is Build with an analysis cache: files whose size and mtime
// match a cache entry reuse its symbols and term frequencies instead of
// being re-analyzed. A nil cache disables caching.
func BuildCached(files []File, c *Cache) *Index {
	ix := &Index{bm25: index.New(), files: files, byPath: map[string]int{}, symbols: map[string][]int{}}
	ix.bm25.Analyze = Analyze
	for i := range files {
		f := &files[i]
		e, hit := c.lookup(f)
		if hit {
			f.Symbols = e.Symbols
		} else if f.Symbols == nil {
			f.Symbols = parseSymbols(f.Content)
		}
		var title strings.Builder
		title.WriteString(strings.TrimSuffix(f.Path, filepath.Ext(f.Path)))
		for _, s := range f.Symbols {
			title.WriteByte(' ')
			title.WriteString(s.Name)
			k := strings.ToLower(s.Name)
			if l := ix.symbols[k]; len(l) == 0 || l[len(l)-1] != i {
				ix.symbols[k] = append(l, i)
			}
		}
		doc := index.Document{ID: f.Path, Title: title.String(), Body: f.Content}
		if !hit {
			e.TF, e.Len = ix.bm25.TermFreqs(doc)
			e.Symbols = f.Symbols
			c.store(f, e)
		}
		ix.bm25.AddTF(doc, e.TF, e.Len)
		ix.byPath[f.Path] = i
	}
	c.prune(files)
	return ix
}

// Len returns the number of indexed files.
func (ix *Index) Len() int { return len(ix.files) }

// Result is a ranked file with the evidence behind its rank.
type Result struct {
	Path    string
	Score   float64
	Reasons []string
}

var (
	traceRe  = regexp.MustCompile(`File "([^"]+\.py)", line (\d+)|(?:at |\()([A-Za-z0-9_./@-]+\.(?:js|ts|tsx|jsx|go|rs|rb|php|java|kt|scala|cs|c|cc|cpp)):(\d+)`)
	dottedRe = regexp.MustCompile(`\b[A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)+\b`)
	pathRe   = regexp.MustCompile(`[A-Za-z0-9_./-]+\.(?:py|jsx?|tsx?|mjs|go|rs|rb|php|java|kt|scala|cs|c|cc|cpp|h|hpp)\b`)
	identRe  = regexp.MustCompile("`([A-Za-z_][A-Za-z0-9_.]*)(?:\\(\\))?`")
)

// Options toggles the issue-specific signals, for ablations.
type Options struct {
	NoTrace, NoPaths, NoSymbols, NoTestPenalty bool
}

// pathSuffix finds indexed files whose path ends with p (stack traces carry
// absolute site-packages paths; issues mention partial paths).
func (ix *Index) pathSuffix(p string) []int {
	p = strings.TrimPrefix(filepath.ToSlash(p), "./")
	var out []int
	for {
		if i, ok := ix.byPath[p]; ok {
			return append(out, i)
		}
		slash := strings.IndexByte(p, '/')
		if slash < 0 {
			return out
		}
		p = p[slash+1:]
		if !strings.Contains(p, "/") { // don't match on a bare filename like "base.py"
			return out
		}
	}
}

// Locate ranks files for an issue report: BM25 over the whole text, plus
// boosts for stack-trace frames, mentioned paths, dotted module paths and
// backticked symbols, minus a penalty for test files.
func (ix *Index) Locate(issue string, k int, opt Options) []Result {
	scores := map[int]float64{}
	reasons := map[int][]string{}
	add := func(i int, s float64, why string) {
		scores[i] += s
		if len(reasons[i]) < 4 {
			reasons[i] = append(reasons[i], why)
		}
	}

	hits := ix.bm25.Search(issue, 200)
	top := 1.0
	if len(hits) > 0 {
		top = hits[0].Score
	}
	for _, h := range hits {
		add(ix.byPath[h.Doc.ID], h.Score/top, "text match")
	}

	if !opt.NoTrace {
		frames := traceRe.FindAllStringSubmatch(issue, -1)
		for n, m := range frames {
			path, line := m[1], m[2]
			if path == "" {
				path, line = m[3], m[4]
			}
			for _, i := range ix.pathSuffix(path) {
				// Deeper frames (closer to the raise) are better evidence.
				add(i, 0.5+0.5*float64(n+1)/float64(len(frames)), "in stack trace line "+line)
			}
		}
	}
	if !opt.NoPaths {
		for _, p := range pathRe.FindAllString(issue, -1) {
			for _, i := range ix.pathSuffix(p) {
				add(i, 0.6, "path mentioned: "+p)
			}
		}
		for _, d := range dottedRe.FindAllString(issue, -1) {
			parts := strings.Split(d, ".")
			for n := len(parts); n >= 2; n-- {
				cand := strings.Join(parts[:n], "/")
				if is := ix.pathSuffix(cand + ".py"); len(is) > 0 {
					for _, i := range is {
						add(i, 0.5, "module mentioned: "+d)
					}
					break
				}
				if is := ix.pathSuffix(cand + "/__init__.py"); len(is) > 0 {
					for _, i := range is {
						add(i, 0.2, "package mentioned: "+d)
					}
					break
				}
			}
		}
	}
	if !opt.NoSymbols {
		seen := map[string]bool{}
		for _, m := range identRe.FindAllStringSubmatch(issue, -1) {
			parts := strings.Split(m[1], ".")
			name := strings.ToLower(parts[len(parts)-1])
			if seen[name] || len(name) < 3 {
				continue
			}
			seen[name] = true
			defs := ix.symbols[name]
			if len(defs) == 0 || len(defs) > 8 { // ubiquitous names carry no signal
				continue
			}
			for _, i := range defs {
				add(i, 0.4/float64(len(defs)), "defines "+parts[len(parts)-1])
			}
		}
	}

	order := make([]int, 0, len(scores))
	for i := range scores {
		if !opt.NoTestPenalty && IsTest(ix.files[i].Path) {
			scores[i] *= 0.3
		}
		order = append(order, i)
	}
	sort.Slice(order, func(a, b int) bool {
		if scores[order[a]] != scores[order[b]] {
			return scores[order[a]] > scores[order[b]]
		}
		return ix.files[order[a]].Path < ix.files[order[b]].Path
	})
	if len(order) > k {
		order = order[:k]
	}
	out := make([]Result, len(order))
	for n, i := range order {
		out[n] = Result{Path: ix.files[i].Path, Score: scores[i], Reasons: reasons[i]}
	}
	return out
}

// Snippet returns up to n line numbers and lines of f that best match query
// terms, so an agent can decide whether to open the file at all.
func (ix *Index) Snippet(path, query string, n int) []string {
	i, ok := ix.byPath[path]
	if !ok {
		return nil
	}
	terms := map[string]bool{}
	for _, t := range Analyze(query) {
		terms[t] = true
	}
	type scored struct {
		line  int
		text  string
		score int
	}
	var lines []scored
	sc := bufio.NewScanner(bytes.NewReader([]byte(ix.files[i].Content)))
	sc.Buffer(make([]byte, 1<<20), 16<<20)
	for ln := 1; sc.Scan(); ln++ {
		s := 0
		seen := map[string]bool{}
		for _, t := range Analyze(sc.Text()) {
			if terms[t] && !seen[t] {
				s++
				seen[t] = true
			}
		}
		if s > 0 {
			lines = append(lines, scored{ln, strings.TrimSpace(sc.Text()), s})
		}
	}
	sort.SliceStable(lines, func(a, b int) bool { return lines[a].score > lines[b].score })
	var out []string
	for _, l := range lines[:min(n, len(lines))] {
		t := l.text
		if len(t) > 160 {
			t = t[:160] + "…"
		}
		out = append(out, fmt.Sprintf("%d: %s", l.line, t))
	}
	return out
}

// Definitions returns "path:line kind name" for every definition of name.
func (ix *Index) Definitions(name string) []string {
	var out []string
	for _, i := range ix.symbols[strings.ToLower(name)] {
		for _, s := range ix.files[i].Symbols {
			if strings.EqualFold(s.Name, name) {
				out = append(out, fmt.Sprintf("%s:%d %s %s", ix.files[i].Path, s.Line, s.Kind, s.Name))
			}
		}
	}
	return out
}
