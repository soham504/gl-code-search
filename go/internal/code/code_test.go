package code

import (
	"reflect"
	"slices"
	"testing"
)

func TestSplitIdent(t *testing.T) {
	cases := map[string][]string{
		"QuerySet":     {"Query", "Set"},
		"HTTPResponse": {"HTTP", "Response"},
		"utf8Decode":   {"utf", "8", "Decode"},
		"plain":        {"plain"},
	}
	for in, want := range cases {
		if got := splitIdent(in); !reflect.DeepEqual(got, want) {
			t.Errorf("splitIdent(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestAnalyzeMatchesProseToIdentifiers(t *testing.T) {
	code := Analyze("def save_date_field(self): DateField()")
	for _, term := range Analyze("saving a date field") {
		if !slices.Contains(code, term) {
			t.Errorf("query term %q not produced for the identifier side: %q", term, code)
		}
	}
	if slices.Contains(code, "self") || slices.Contains(code, "def") {
		t.Errorf("python noise words kept: %q", code)
	}
}

func TestIsTest(t *testing.T) {
	for path, want := range map[string]bool{
		"tests/test_views.py":     true,
		"pkg/tests/helpers.py":    true,
		"pkg/conftest.py":         true,
		"pkg/views_test.py":       true,
		"django/views/debug.py":   false,
		"pkg/testimonials/api.py": false,
	} {
		if got := IsTest(path); got != want {
			t.Errorf("IsTest(%q) = %v, want %v", path, got, want)
		}
	}
}

func corpus() []File {
	src := map[string]string{
		"app/http/response.py": "class HttpResponse:\n    def set_cookie(self, key):\n        pass\n",
		"app/views/debug.py":   "def technical_404_response(request, exception):\n    return render_404(exception)\n",
		"app/urls/resolvers.py": "class URLResolver:\n    def resolve(self, path):\n" +
			"        raise Resolver404(path)\n",
		"tests/test_debug.py": "def test_technical_404_response():\n    technical_404_response(None, None)\n",
	}
	var files []File
	for p, s := range src {
		files = append(files, File{Path: p, Content: s, Symbols: parseSymbols(s)})
	}
	return files
}

func TestLocateSignals(t *testing.T) {
	ix := Build(corpus())

	got := ix.Locate("The technical 404 response page shows the wrong exception", 3, Options{})
	if got[0].Path != "app/views/debug.py" {
		t.Errorf("top hit = %s, want app/views/debug.py (test file must be penalised)", got[0].Path)
	}

	trace := "Traceback:\n  File \"/usr/lib/site-packages/app/urls/resolvers.py\", line 3, in resolve\n"
	got = ix.Locate(trace, 1, Options{})
	if got[0].Path != "app/urls/resolvers.py" {
		t.Errorf("stack trace frame not boosted: top = %s", got[0].Path)
	}

	got = ix.Locate("calling `set_cookie()` fails", 1, Options{})
	if got[0].Path != "app/http/response.py" {
		t.Errorf("backticked symbol not resolved: top = %s", got[0].Path)
	}
}

func TestDefinitions(t *testing.T) {
	ix := Build(corpus())
	defs := ix.Definitions("resolve")
	if len(defs) != 1 || defs[0] != "app/urls/resolvers.py:2 def resolve" {
		t.Errorf("Definitions(resolve) = %q", defs)
	}
}

func TestParseSymbolsMultiLanguage(t *testing.T) {
	src := `export default class Router {}
export async function handleRequest(req) {}
func (s *Server) ServeHTTP(w, r) {}
pub(crate) fn parse_header(b: &[u8]) {}
public final class HttpClient {
  def initialize(x)
interface Props {}
`
	var names []string
	for _, s := range parseSymbols(src) {
		names = append(names, s.Name)
	}
	want := []string{"Router", "handleRequest", "ServeHTTP", "parse_header", "HttpClient", "initialize", "Props"}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("parseSymbols = %q, want %q", names, want)
	}
}

func TestIndexableAndTestPaths(t *testing.T) {
	for path, want := range map[string]bool{
		"src/router.ts": true, "lib/a.min.js": false, "types/index.d.ts": false,
		"node_modules/x/y.js": false, "README.md": false, "cmd/main.go": true,
	} {
		if got := Indexable(path); got != want {
			t.Errorf("Indexable(%q) = %v, want %v", path, got, want)
		}
	}
	for path, want := range map[string]bool{
		"src/__tests__/router.ts": true, "src/router.test.ts": true, "pkg/server_test.go": true,
		"src/test/java/FooTest.java": true, "spec/models/user_spec.rb": true, "src/specialist.ts": false,
	} {
		if got := IsTest(path); got != want {
			t.Errorf("IsTest(%q) = %v, want %v", path, got, want)
		}
	}
}
