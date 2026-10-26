package index

import "github.com/kljensen/snowball/english"

// Analyzer turns text into index terms.
type Analyzer func(string) []string

// stopwords is the Lucene English stop set, which BEIR's BM25 baseline uses.
var stopwords = map[string]bool{}

func init() {
	for _, w := range []string{"a", "an", "and", "are", "as", "at", "be", "but", "by", "for", "if", "in", "into",
		"is", "it", "no", "not", "of", "on", "or", "such", "that", "the", "their", "then", "there", "these",
		"they", "this", "to", "was", "will", "with"} {
		stopwords[w] = true
	}
}

// English drops stopwords and applies the Snowball English stemmer.
// Non-Latin tokens pass through the stemmer unchanged.
func English(s string) []string {
	toks := Tokenize(s)
	out := toks[:0]
	for _, t := range toks {
		if stopwords[t] {
			continue
		}
		out = append(out, english.Stem(t, false))
	}
	return out
}
