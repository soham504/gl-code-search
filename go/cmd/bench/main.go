// Command bench evaluates BM25 on a BEIR dataset against its official qrels,
// reports nDCG@10 and query latency, and dumps top-k evidence for a seeded
// sample of queries (the input to the grounding pilot).
//
//	bench -beir data/fiqa -sample 50 -evidence eval/pilot/fiqa_evidence.jsonl
package main

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"github.com/soham504/gl-code-search/go/internal/index"
)

type beirDoc struct {
	ID    string `json:"_id"`
	Title string `json:"title"`
	Text  string `json:"text"`
}

func readJSONL(path string, fn func([]byte) error) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 64<<20)
	for sc.Scan() {
		if err := fn(sc.Bytes()); err != nil {
			return err
		}
	}
	return sc.Err()
}

func readQrels(path string) (map[string]map[string]float64, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	if _, err := r.Read(); err != nil { // header
		return nil, err
	}
	q := map[string]map[string]float64{}
	for {
		rec, err := r.Read()
		if err == io.EOF {
			return q, nil
		}
		if err != nil {
			return nil, err
		}
		score, _ := strconv.ParseFloat(rec[2], 64)
		if q[rec[0]] == nil {
			q[rec[0]] = map[string]float64{}
		}
		q[rec[0]][rec[1]] = score
	}
}

func ndcg10(ranked []index.Hit, rel map[string]float64) float64 {
	var dcg, idcg float64
	for i, h := range ranked {
		if i == 10 {
			break
		}
		dcg += rel[h.Doc.ID] / math.Log2(float64(i+2))
	}
	ideal := make([]float64, 0, len(rel))
	for _, g := range rel {
		ideal = append(ideal, g)
	}
	sort.Sort(sort.Reverse(sort.Float64Slice(ideal)))
	for i, g := range ideal {
		if i == 10 {
			break
		}
		idcg += g / math.Log2(float64(i+2))
	}
	if idcg == 0 {
		return 0
	}
	return dcg / idcg
}

func pct(d []time.Duration, p float64) time.Duration {
	return d[int(math.Ceil(p*float64(len(d))))-1]
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

func main() {
	dir := flag.String("beir", "data/fiqa", "BEIR dataset directory")
	split := flag.String("split", "test", "qrels split")
	analyzer := flag.String("analyzer", "english", "english or plain")
	sample := flag.Int("sample", 0, "number of queries to dump evidence for")
	k := flag.Int("k", 5, "evidence passages per sampled query")
	evidence := flag.String("evidence", "", "evidence output JSONL")
	seed := flag.Int64("seed", 42, "sampling seed")
	flag.Parse()

	ix := index.New()
	if *analyzer == "english" {
		ix.Analyze = index.English
	}
	name := filepath.Base(*dir)
	source := "web"
	if name == "fiqa" {
		source = "forum" // FiQA's corpus is StackExchange and Reddit posts
	}

	start := time.Now()
	err := readJSONL(filepath.Join(*dir, "corpus.jsonl"), func(b []byte) error {
		var d beirDoc
		if err := json.Unmarshal(b, &d); err != nil {
			return err
		}
		ix.Add(index.Document{ID: d.ID, Title: d.Title, Body: d.Text, Source: source})
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}
	buildTime := time.Since(start)

	queries := map[string]string{}
	if err := readJSONL(filepath.Join(*dir, "queries.jsonl"), func(b []byte) error {
		var d beirDoc
		if err := json.Unmarshal(b, &d); err != nil {
			return err
		}
		queries[d.ID] = d.Text
		return nil
	}); err != nil {
		log.Fatal(err)
	}
	qrels, err := readQrels(filepath.Join(*dir, "qrels", *split+".tsv"))
	if err != nil {
		log.Fatal(err)
	}

	qids := make([]string, 0, len(qrels))
	for id := range qrels {
		qids = append(qids, id)
	}
	sort.Strings(qids)

	var sum float64
	lat := make([]time.Duration, 0, len(qids))
	runs := make(map[string][]index.Hit, len(qids))
	for _, id := range qids {
		t := time.Now()
		hits := ix.Search(queries[id], 100)
		lat = append(lat, time.Since(t))
		runs[id] = hits
		sum += ndcg10(hits, qrels[id])
	}
	sort.Slice(lat, func(i, j int) bool { return lat[i] < lat[j] })

	report := map[string]any{
		"dataset":     name,
		"analyzer":    *analyzer,
		"docs":        ix.Len(),
		"queries":     len(qids),
		"ndcg@10":     math.Round(sum/float64(len(qids))*10000) / 10000,
		"index_build": buildTime.Round(time.Millisecond).String(),
		"latency_p50": pct(lat, 0.50).String(),
		"latency_p99": pct(lat, 0.99).String(),
		"latency_mean": (func() time.Duration {
			var s time.Duration
			for _, l := range lat {
				s += l
			}
			return s / time.Duration(len(lat))
		})().String(),
	}
	out, _ := json.Marshal(report)
	fmt.Println(string(out))

	if *sample == 0 || *evidence == "" {
		return
	}
	rng := rand.New(rand.NewSource(*seed))
	rng.Shuffle(len(qids), func(i, j int) { qids[i], qids[j] = qids[j], qids[i] })
	if err := os.MkdirAll(filepath.Dir(*evidence), 0o755); err != nil {
		log.Fatal(err)
	}
	f, err := os.Create(*evidence)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	for _, id := range qids[:min(*sample, len(qids))] {
		type passage struct {
			ID, Text string
			Relevant bool
		}
		ps := []passage{}
		for _, h := range runs[id][:min(*k, len(runs[id]))] {
			ps = append(ps, passage{h.Doc.ID, truncate(h.Doc.Title+" "+h.Doc.Body, 1500), qrels[id][h.Doc.ID] > 0})
		}
		enc.Encode(map[string]any{"dataset": name, "source": source, "qid": id, "query": queries[id], "passages": ps})
	}
}
