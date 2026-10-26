// Command searchd serves BM25 search over a JSONL corpus as JSON and an
// accessible HTML results page.
//
//	searchd -corpus eval/sample_corpus.jsonl -addr :8080
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/soham504/gl-code-search/go/internal/index"
	"github.com/soham504/gl-code-search/go/internal/serp"
)

func loadCorpus(path string) (*index.Index, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	ix := index.New()
	ix.Analyze = index.English // same analyzer as the BEIR benchmark
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 16<<20)
	for line := 1; sc.Scan(); line++ {
		var d index.Document
		if err := json.Unmarshal(sc.Bytes(), &d); err != nil {
			return nil, fmt.Errorf("%s:%d: %w", path, line, err)
		}
		ix.Add(d)
	}
	return ix, sc.Err()
}

func main() {
	corpus := flag.String("corpus", "eval/sample_corpus.jsonl", "JSONL file of documents")
	addr := flag.String("addr", ":8080", "listen address")
	flag.Parse()

	start := time.Now()
	ix, err := loadCorpus(*corpus)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("indexed %d docs in %v", ix.Len(), time.Since(start))

	http.HandleFunc("/api/search", func(w http.ResponseWriter, r *http.Request) {
		k, _ := strconv.Atoi(r.URL.Query().Get("k"))
		if k <= 0 || k > 100 {
			k = 10
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ix.Search(r.URL.Query().Get("q"), k))
	})
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		var hits []index.Hit
		if q != "" {
			hits = ix.Search(q, 10)
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := serp.Render(w, serp.Data{Query: q, Hits: hits}); err != nil {
			log.Printf("render: %v", err)
		}
	})

	log.Printf("listening on %s", *addr)
	log.Fatal(http.ListenAndServe(*addr, nil))
}
