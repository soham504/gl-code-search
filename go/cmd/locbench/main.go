// Command locbench measures file-level issue localization: for each
// instance, index the repo at its base commit, rank files for the issue text,
// and record where the first file changed by the gold patch lands.
//
//	locbench -data data/bench/swebench_verified.jsonl -repos data/repos -name verified -j 8
//
// With -fetch, missing repositories are created as bare repos and only the
// needed commit is fetched (git fetch --depth 1 origin <sha>), so benchmarks
// spanning thousands of repositories don't require full clones.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/soham504/gl-code-search/go/internal/code"
	"github.com/soham504/gl-code-search/go/internal/index"
)

type instance struct {
	ID      string   `json:"instance_id"`
	Repo    string   `json:"repo"`
	Base    string   `json:"base_commit"`
	Problem string   `json:"problem_statement"`
	Gold    []string `json:"gold_files"`
	Lang    string   `json:"lang"`
}

type system struct {
	name string
	rank func(files []code.File, ix *code.Index, issue string) []string
}

// plainBM25 is the conventional baseline: English analyzer, path as title, no
// code-aware tokenization and no issue-specific signals.
func plainBM25(files []code.File, _ *code.Index, issue string) []string {
	ix := index.New()
	ix.Analyze = index.English
	for _, f := range files {
		ix.Add(index.Document{ID: f.Path, Title: f.Path, Body: f.Content})
	}
	var out []string
	for _, h := range ix.Search(issue, 100) {
		out = append(out, h.Doc.ID)
	}
	return out
}

func codeRanker(opt code.Options) func([]code.File, *code.Index, string) []string {
	return func(_ []code.File, ix *code.Index, issue string) []string {
		var out []string
		for _, r := range ix.Locate(issue, 100, opt) {
			out = append(out, r.Path)
		}
		return out
	}
}

var systems = []system{
	{"bm25_plain", plainBM25},
	{"gl_text_only", codeRanker(code.Options{NoTrace: true, NoPaths: true, NoSymbols: true, NoTestPenalty: true})},
	{"gl_no_test_penalty", codeRanker(code.Options{NoTestPenalty: true})},
	{"gl_no_trace_paths", codeRanker(code.Options{NoTrace: true, NoPaths: true})},
	{"gl_no_symbols", codeRanker(code.Options{NoSymbols: true})},
	{"gl_full", codeRanker(code.Options{})},
}

var repoLocks sync.Map // gitDir -> *sync.Mutex; git fetches into one repo must not overlap

func ensureCommit(gitDir, repo, sha string, fetch bool) error {
	if exec.Command("git", "--git-dir", gitDir, "cat-file", "-e", sha+"^{commit}").Run() == nil {
		return nil
	}
	if !fetch {
		return fmt.Errorf("commit %s not in %s (use -fetch)", sha, gitDir)
	}
	mu, _ := repoLocks.LoadOrStore(gitDir, &sync.Mutex{})
	mu.(*sync.Mutex).Lock()
	defer mu.(*sync.Mutex).Unlock()
	if _, err := os.Stat(gitDir); err != nil {
		if out, err := exec.Command("git", "init", "-q", "--bare", gitDir).CombinedOutput(); err != nil {
			return fmt.Errorf("git init: %v: %s", err, out)
		}
		exec.Command("git", "--git-dir", gitDir, "remote", "add", "origin", "https://github.com/"+repo+".git").Run()
	}
	out, err := exec.Command("git", "--git-dir", gitDir, "fetch", "-q", "--depth", "1", "origin", sha).CombinedOutput()
	if err != nil {
		return fmt.Errorf("fetch %s@%s: %v: %s", repo, sha, err, strings.TrimSpace(string(out)))
	}
	return nil
}

type result struct {
	row   map[string]any
	ranks map[string]int
	lat   map[string]time.Duration
	err   error
}

func evaluate(in instance, reposDir string, fetch bool) result {
	gitDir := filepath.Join(reposDir, strings.ReplaceAll(in.Repo, "/", "_")+".git")
	if err := ensureCommit(gitDir, in.Repo, in.Base, fetch); err != nil {
		return result{err: err}
	}
	files, err := code.LoadGit(gitDir, in.Base)
	if err != nil {
		return result{err: err}
	}
	t0 := time.Now()
	ix := code.Build(files)
	build := time.Since(t0)
	res := result{
		row:   map[string]any{"instance_id": in.ID, "repo": in.Repo, "lang": in.Lang, "files": len(files), "gold": in.Gold},
		ranks: map[string]int{},
		lat:   map[string]time.Duration{},
	}
	// Evidence behind gl's top results, for calibrating a confidence gate offline.
	var reasons [][]string
	var scores []float64
	for _, r := range ix.Locate(in.Problem, 6, code.Options{}) {
		reasons = append(reasons, r.Reasons)
		scores = append(scores, r.Score)
	}
	res.row["gl_top_reasons"], res.row["gl_top_scores"] = reasons, scores
	for _, s := range systems {
		t := time.Now()
		ranked := s.rank(files, ix, in.Problem)
		d := time.Since(t)
		if s.name != "bm25_plain" {
			d += build // charge the shared code index build to every gl variant
		}
		rank := 0 // rank of the first gold file (instances may change several files)
		for i, p := range ranked {
			if slices.Contains(in.Gold, p) {
				rank = i + 1
				break
			}
		}
		res.ranks[s.name], res.lat[s.name] = rank, d
		res.row[s.name] = rank
		res.row[s.name+"_top5"] = ranked[:min(5, len(ranked))]
	}
	return res
}

func main() {
	data := flag.String("data", "data/swebench/lite.jsonl", "instances JSONL with gold_files")
	repos := flag.String("repos", "data/repos", "directory of <owner>_<name>.git bare repos")
	name := flag.String("name", "", "benchmark name for the report (default: data file name)")
	out := flag.String("out", "", "per-instance results JSONL (default eval/results/loc_<name>.jsonl)")
	sample := flag.Int("sample", 0, "evaluate a seeded random sample of this many instances (0 = all)")
	seed := flag.Int64("seed", 42, "sampling seed")
	workers := flag.Int("j", 4, "parallel workers")
	fetch := flag.Bool("fetch", false, "fetch missing repos/commits from GitHub")
	flag.Parse()
	if *name == "" {
		*name = strings.TrimSuffix(filepath.Base(*data), ".jsonl")
	}
	if *out == "" {
		*out = "eval/results/loc_" + *name + ".jsonl"
	}

	f, err := os.Open(*data)
	if err != nil {
		log.Fatal(err)
	}
	var insts []instance
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 64<<20)
	for sc.Scan() {
		var in instance
		if err := json.Unmarshal(sc.Bytes(), &in); err != nil {
			log.Fatal(err)
		}
		insts = append(insts, in)
	}
	f.Close()
	if *sample > 0 && *sample < len(insts) {
		rand.New(rand.NewSource(*seed)).Shuffle(len(insts), func(i, j int) { insts[i], insts[j] = insts[j], insts[i] })
		insts = insts[:*sample]
	}

	jobs := make(chan instance)
	results := make(chan result)
	var wg sync.WaitGroup
	for w := 0; w < *workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for in := range jobs {
				r := evaluate(in, *repos, *fetch)
				if r.err != nil {
					r.err = fmt.Errorf("%s: %w", in.ID, r.err)
				}
				results <- r
			}
		}()
	}
	go func() {
		for _, in := range insts {
			jobs <- in
		}
		close(jobs)
		wg.Wait()
		close(results)
	}()

	var rows []map[string]any
	ks := []int{1, 3, 5, 10}
	hits := map[string]map[int]int{}
	lat := map[string][]time.Duration{}
	for _, s := range systems {
		hits[s.name] = map[int]int{}
	}
	skipped := 0
	for r := range results {
		if r.err != nil {
			skipped++
			log.Printf("skip %v", r.err)
			continue
		}
		rows = append(rows, r.row)
		for _, s := range systems {
			lat[s.name] = append(lat[s.name], r.lat[s.name])
			for _, k := range ks {
				if rk := r.ranks[s.name]; rk > 0 && rk <= k {
					hits[s.name][k]++
				}
			}
		}
		if len(rows)%100 == 0 {
			log.Printf("%s: %d/%d", *name, len(rows), len(insts))
		}
	}
	if len(rows) == 0 {
		log.Fatalf("%s: no instances evaluated (%d skipped)", *name, skipped)
	}

	sort.Slice(rows, func(i, j int) bool { return rows[i]["instance_id"].(string) < rows[j]["instance_id"].(string) })
	w, err := os.Create(*out)
	if err != nil {
		log.Fatal(err)
	}
	enc := json.NewEncoder(w)
	for _, r := range rows {
		enc.Encode(r)
	}
	w.Close()

	n := len(rows)
	fmt.Printf("## %s: file localization, n=%d", *name, n)
	if skipped > 0 {
		fmt.Printf(" (%d skipped: repo/commit unavailable)", skipped)
	}
	fmt.Print("\n\n| System | Acc@1 | Acc@3 | Acc@5 | Acc@10 | p50 index+rank | p99 |\n|---|---|---|---|---|---|---|\n")
	for _, s := range systems {
		l := lat[s.name]
		sort.Slice(l, func(i, j int) bool { return l[i] < l[j] })
		fmt.Printf("| %s |", s.name)
		for _, k := range ks {
			fmt.Printf(" %.1f%% |", 100*float64(hits[s.name][k])/float64(n))
		}
		fmt.Printf(" %v | %v |\n", l[n/2].Round(time.Millisecond), l[min(n-1, n*99/100)].Round(time.Millisecond))
	}
}
