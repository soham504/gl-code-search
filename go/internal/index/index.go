// Package index implements an in-memory inverted index with BM25 scoring.
package index

import (
	"math"
	"sort"
	"strings"
	"unicode"
)

// Document is a unit of retrieval. Source distinguishes web pages from
// forum threads and transcripts, which the claim verifier treats differently.
type Document struct {
	ID     string
	Title  string
	URL    string
	Body   string
	Source string
}

type posting struct {
	doc int // internal doc number
	tf  int
}

// Index is an inverted index. It is not safe for concurrent writes;
// build it once, then serve reads.
type Index struct {
	docs     []Document
	docLen   []int
	totalLen int
	postings map[string][]posting

	K1      float64
	B       float64
	Analyze Analyzer
}

// New returns an empty index with standard BM25 parameters.
func New() *Index {
	return &Index{postings: make(map[string][]posting), K1: 1.2, B: 0.75, Analyze: Tokenize}
}

// Tokenize lowercases and splits on anything that is not a letter or digit.
// It is Unicode-aware so Devanagari and Bengali text tokenize correctly.
func Tokenize(s string) []string {
	return strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && !unicode.Is(unicode.Mn, r) && !unicode.Is(unicode.Mc, r)
	})
}

// Add indexes a document. Title terms are counted twice as a cheap field boost.
func (ix *Index) Add(d Document) {
	tf, length := ix.TermFreqs(d)
	ix.AddTF(d, tf, length)
}

// TermFreqs analyzes d the way Add does, returning term frequencies and the
// document length, so callers can cache the expensive part.
func (ix *Index) TermFreqs(d Document) (map[string]int, int) {
	terms := append(ix.Analyze(d.Title), ix.Analyze(d.Title+" "+d.Body)...)
	tf := make(map[string]int, len(terms))
	for _, t := range terms {
		tf[t]++
	}
	return tf, len(terms)
}

// AddTF indexes a document from precomputed TermFreqs output.
func (ix *Index) AddTF(d Document, tf map[string]int, length int) {
	n := len(ix.docs)
	ix.docs = append(ix.docs, d)
	for t, c := range tf {
		ix.postings[t] = append(ix.postings[t], posting{doc: n, tf: c})
	}
	ix.docLen = append(ix.docLen, length)
	ix.totalLen += length
}

// Len returns the number of indexed documents.
func (ix *Index) Len() int { return len(ix.docs) }

// Hit is a scored search result.
type Hit struct {
	Doc   Document
	Score float64
}

func (ix *Index) idf(df int) float64 {
	n := float64(len(ix.docs))
	return math.Log(1 + (n-float64(df)+0.5)/(float64(df)+0.5))
}

// Search returns the top k documents for query by BM25 score.
// Ties are broken by insertion order so results are deterministic.
func (ix *Index) Search(query string, k int) []Hit {
	if len(ix.docs) == 0 || k <= 0 {
		return nil
	}
	avgdl := float64(ix.totalLen) / float64(len(ix.docs))
	scores := make(map[int]float64)

	seen := make(map[string]bool)
	for _, t := range ix.Analyze(query) {
		if seen[t] {
			continue
		}
		seen[t] = true
		pl := ix.postings[t]
		if len(pl) == 0 {
			continue
		}
		idf := ix.idf(len(pl))
		for _, p := range pl {
			tf := float64(p.tf)
			norm := ix.K1 * (1 - ix.B + ix.B*float64(ix.docLen[p.doc])/avgdl)
			scores[p.doc] += idf * tf * (ix.K1 + 1) / (tf + norm)
		}
	}

	hits := make([]Hit, 0, len(scores))
	order := make([]int, 0, len(scores))
	for d := range scores {
		order = append(order, d)
	}
	sort.Slice(order, func(i, j int) bool {
		if scores[order[i]] != scores[order[j]] {
			return scores[order[i]] > scores[order[j]]
		}
		return order[i] < order[j]
	})
	if len(order) > k {
		order = order[:k]
	}
	for _, d := range order {
		hits = append(hits, Hit{Doc: ix.docs[d], Score: scores[d]})
	}
	return hits
}
