// Package serp renders an accessible search results page.
//
// Design rules, from the screen-reader SERP studies cited in the README:
// results come first in source order, each result is a list item with a
// heading, and a live region announces the result count.
package serp

import (
	"html/template"
	"io"

	"github.com/soham504/gl-code-search/go/internal/index"
)

var page = template.Must(template.New("serp").Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{if .Query}}{{.Query}} – {{end}}Groundline</title>
<style>
  body { font: 16px/1.5 system-ui, sans-serif; max-width: 46rem; margin: 0 auto; padding: 1rem; color: #1a1a1a; background: #fff; }
  a { color: #0b57d0; } a:focus, input:focus, button:focus { outline: 3px solid #0b57d0; outline-offset: 2px; }
  .skip { position: absolute; left: -999px; } .skip:focus { left: 1rem; top: 1rem; background: #fff; }
  ol { padding: 0; list-style: none; } li { margin-block: 1.25rem; }
  h2 { font-size: 1.1rem; margin: 0; } .url { color: #3c4043; font-size: .875rem; }
  .src { font-size: .75rem; border: 1px solid #777; border-radius: 4px; padding: 0 .3rem; }
</style>
</head>
<body>
<a class="skip" href="#results">Skip to results</a>
<main>
  <h1>Groundline</h1>
  <form role="search" action="/search" method="get">
    <label for="q">Search</label>
    <input id="q" name="q" type="search" value="{{.Query}}" autocomplete="off">
    <button type="submit">Search</button>
  </form>
  {{if .Query}}
  <section id="results" aria-labelledby="results-heading">
    <h2 id="results-heading" aria-live="polite">{{len .Hits}} results for “{{.Query}}”</h2>
    <ol>
    {{range .Hits}}
      <li>
        <h2><a href="{{.Doc.URL}}">{{.Doc.Title}}</a></h2>
        <div class="url">{{.Doc.URL}}{{if .Doc.Source}} <span class="src">{{.Doc.Source}}</span>{{end}}</div>
      </li>
    {{end}}
    </ol>
  </section>
  {{end}}
</main>
</body>
</html>
`))

// Data is the template input.
type Data struct {
	Query string
	Hits  []index.Hit
}

// Render writes the page for d to w.
func Render(w io.Writer, d Data) error { return page.Execute(w, d) }
