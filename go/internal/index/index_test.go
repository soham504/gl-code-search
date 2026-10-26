package index

import (
	"reflect"
	"testing"
)

func TestTokenize(t *testing.T) {
	cases := map[string][]string{
		"Hello, World!":    {"hello", "world"},
		"BM25 rocks--2026": {"bm25", "rocks", "2026"},
		"खोज इंजन":         {"खोज", "इंजन"},
		"":                 nil,
	}
	for in, want := range cases {
		got := Tokenize(in)
		if len(got) == 0 && len(want) == 0 {
			continue
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("Tokenize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSearchRanksRelevantFirst(t *testing.T) {
	ix := New()
	ix.Add(Document{ID: "a", Title: "Cooking pasta", Body: "boil water, add salt"})
	ix.Add(Document{ID: "b", Title: "Inverted index", Body: "an inverted index maps terms to postings for search"})
	ix.Add(Document{ID: "c", Title: "Search engines", Body: "search engines rank documents"})

	hits := ix.Search("inverted index search", 10)
	if len(hits) != 2 {
		t.Fatalf("got %d hits, want 2", len(hits))
	}
	if hits[0].Doc.ID != "b" {
		t.Errorf("top hit = %s, want b", hits[0].Doc.ID)
	}
	if hits[0].Score <= hits[1].Score {
		t.Errorf("scores not descending: %v", hits)
	}
}

func TestSearchEdgeCases(t *testing.T) {
	ix := New()
	if got := ix.Search("anything", 5); got != nil {
		t.Errorf("empty index returned %v", got)
	}
	ix.Add(Document{ID: "a", Body: "alpha"})
	if got := ix.Search("beta", 5); len(got) != 0 {
		t.Errorf("unmatched query returned %v", got)
	}
	if got := ix.Search("alpha", 0); got != nil {
		t.Errorf("k=0 returned %v", got)
	}
}

func TestDuplicateQueryTermsDoNotDoubleCount(t *testing.T) {
	ix := New()
	ix.Add(Document{ID: "a", Body: "go go gophers"})
	ix.Add(Document{ID: "b", Body: "rust crabs"})
	one := ix.Search("go", 1)[0].Score
	two := ix.Search("go go", 1)[0].Score
	if one != two {
		t.Errorf("repeated query term changed score: %v vs %v", one, two)
	}
}
