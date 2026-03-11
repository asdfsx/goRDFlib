package sparqlstore_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/sparql"
	"github.com/tggo/goRDFlib/store/sparqlstore"
	"github.com/tggo/goRDFlib/term"
)

// newTestServer creates an httptest.Server backed by an in-memory graph and
// returns the server, a remote SPARQLStore connected to it, and a cleanup func.
func newTestServer(t *testing.T) (*httptest.Server, *sparqlstore.SPARQLStore) {
	t.Helper()
	g := graph.NewGraph()
	ds := &sparql.Dataset{
		Default:     g,
		NamedGraphs: make(map[string]*rdflibgo.Graph),
	}
	srv := sparqlstore.NewServer(ds)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	remote := sparqlstore.New(
		ts.URL+"/query",
		sparqlstore.WithUpdate(ts.URL+"/update"),
	)
	return ts, remote
}

func TestAddAndTriples(t *testing.T) {
	_, store := newTestServer(t)

	s := term.NewURIRefUnsafe("http://example.org/s")
	p := term.NewURIRefUnsafe("http://example.org/p")
	o := rdflibgo.NewLiteral("hello")

	store.Add(term.Triple{Subject: s, Predicate: p, Object: o}, nil)

	var count int
	for range store.Triples(term.TriplePattern{}, nil) {
		count++
	}
	if count != 1 {
		t.Fatalf("expected 1 triple, got %d", count)
	}

	// Check the actual values
	for tr := range store.Triples(term.TriplePattern{Subject: s}, nil) {
		if !tr.Subject.Equal(s) {
			t.Errorf("subject = %v, want %v", tr.Subject, s)
		}
		if !tr.Predicate.Equal(p) {
			t.Errorf("predicate = %v, want %v", tr.Predicate, p)
		}
		if !tr.Object.Equal(o) {
			t.Errorf("object = %v, want %v", tr.Object, o)
		}
	}
}

func TestLen(t *testing.T) {
	_, store := newTestServer(t)

	s := term.NewURIRefUnsafe("http://example.org/s")
	p := term.NewURIRefUnsafe("http://example.org/p")

	store.Add(term.Triple{Subject: s, Predicate: p, Object: rdflibgo.NewLiteral("one")}, nil)
	store.Add(term.Triple{Subject: s, Predicate: p, Object: rdflibgo.NewLiteral("two")}, nil)

	if n := store.Len(nil); n != 2 {
		t.Fatalf("Len = %d, want 2", n)
	}
}

func TestRemove(t *testing.T) {
	_, store := newTestServer(t)

	s := term.NewURIRefUnsafe("http://example.org/s")
	p := term.NewURIRefUnsafe("http://example.org/p")
	o := rdflibgo.NewLiteral("hello")

	store.Add(term.Triple{Subject: s, Predicate: p, Object: o}, nil)
	store.Remove(term.TriplePattern{Subject: s, Predicate: &p, Object: o}, nil)

	if n := store.Len(nil); n != 0 {
		t.Fatalf("Len after remove = %d, want 0", n)
	}
}

func TestSet(t *testing.T) {
	_, store := newTestServer(t)

	s := term.NewURIRefUnsafe("http://example.org/s")
	p := term.NewURIRefUnsafe("http://example.org/p")

	store.Add(term.Triple{Subject: s, Predicate: p, Object: rdflibgo.NewLiteral("old")}, nil)
	store.Set(term.Triple{Subject: s, Predicate: p, Object: rdflibgo.NewLiteral("new")}, nil)

	if n := store.Len(nil); n != 1 {
		t.Fatalf("Len after set = %d, want 1", n)
	}

	for tr := range store.Triples(term.TriplePattern{Subject: s}, nil) {
		lit, ok := tr.Object.(term.Literal)
		if !ok {
			t.Fatalf("object is not Literal: %T", tr.Object)
		}
		if lit.Value() != "new" {
			t.Errorf("object value = %q, want %q", lit.Value(), "new")
		}
	}
}

func TestNamespaces(t *testing.T) {
	_, store := newTestServer(t)

	ns := term.NewURIRefUnsafe("http://example.org/")
	store.Bind("ex", ns)

	got, ok := store.Namespace("ex")
	if !ok || !got.Equal(ns) {
		t.Errorf("Namespace(ex) = %v, %v; want %v, true", got, ok, ns)
	}

	prefix, ok := store.Prefix(ns)
	if !ok || prefix != "ex" {
		t.Errorf("Prefix(%v) = %q, %v; want %q, true", ns, prefix, ok, "ex")
	}

	var nsCount int
	for range store.Namespaces() {
		nsCount++
	}
	if nsCount != 1 {
		t.Errorf("namespace count = %d, want 1", nsCount)
	}
}

func TestContextAware(t *testing.T) {
	_, store := newTestServer(t)
	if !store.ContextAware() {
		t.Error("ContextAware() = false, want true")
	}
	if store.TransactionAware() {
		t.Error("TransactionAware() = true, want false")
	}
}

func TestAddN(t *testing.T) {
	_, store := newTestServer(t)

	s := term.NewURIRefUnsafe("http://example.org/s")
	p := term.NewURIRefUnsafe("http://example.org/p")

	quads := []term.Quad{
		{Triple: term.Triple{Subject: s, Predicate: p, Object: rdflibgo.NewLiteral("a")}},
		{Triple: term.Triple{Subject: s, Predicate: p, Object: rdflibgo.NewLiteral("b")}},
	}
	store.AddN(quads)

	if n := store.Len(nil); n != 2 {
		t.Fatalf("Len after AddN = %d, want 2", n)
	}
}

func TestTriplesWithPattern(t *testing.T) {
	_, store := newTestServer(t)

	s1 := term.NewURIRefUnsafe("http://example.org/s1")
	s2 := term.NewURIRefUnsafe("http://example.org/s2")
	p := term.NewURIRefUnsafe("http://example.org/p")
	o := rdflibgo.NewLiteral("val")

	store.Add(term.Triple{Subject: s1, Predicate: p, Object: o}, nil)
	store.Add(term.Triple{Subject: s2, Predicate: p, Object: o}, nil)

	// Filter by subject
	var count int
	for range store.Triples(term.TriplePattern{Subject: s1}, nil) {
		count++
	}
	if count != 1 {
		t.Errorf("triples with subject s1 = %d, want 1", count)
	}
}

func TestGraphIntegration(t *testing.T) {
	ts, remote := newTestServer(t)
	_ = ts

	// Use the remote store with a graph
	g := graph.NewGraph(graph.WithStore(remote))

	s := term.NewURIRefUnsafe("http://example.org/s")
	p := term.NewURIRefUnsafe("http://example.org/p")
	o := rdflibgo.NewLiteral("graph test")

	g.Add(s, p, o)

	if g.Len() != 1 {
		t.Fatalf("graph Len = %d, want 1", g.Len())
	}

	// Query through SPARQL
	result, err := sparql.Query(g, `SELECT ?o WHERE { <http://example.org/s> <http://example.org/p> ?o }`)
	if err != nil {
		t.Fatalf("SPARQL query error: %v", err)
	}
	if len(result.Bindings) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(result.Bindings))
	}
	if lit, ok := result.Bindings[0]["o"].(rdflibgo.Literal); !ok || lit.Value() != "graph test" {
		t.Errorf("unexpected result: %v", result.Bindings[0]["o"])
	}
}

func TestContexts(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return a mock SRX with one graph URI.
		w.Header().Set("Content-Type", "application/sparql-results+xml")
		w.Write([]byte(`<?xml version="1.0"?>
<sparql xmlns="http://www.w3.org/2005/sparql-results#">
<head><variable name="g"/></head>
<results>
<result><binding name="g"><uri>http://example.org/graph1</uri></binding></result>
</results>
</sparql>`))
	}))
	t.Cleanup(ts.Close)

	remote := sparqlstore.New(ts.URL)
	var count int
	for g := range remote.Contexts(nil) {
		count++
		if u, ok := g.(term.URIRef); !ok || u.Value() != "http://example.org/graph1" {
			t.Errorf("unexpected context: %v", g)
		}
	}
	if count != 1 {
		t.Errorf("Contexts count = %d, want 1", count)
	}
}

func TestContextsWithTriple(t *testing.T) {
	_, store := newTestServer(t)

	// Contexts with a specific triple filter
	s := term.NewURIRefUnsafe("http://example.org/s")
	p := term.NewURIRefUnsafe("http://example.org/p")
	o := rdflibgo.NewLiteral("hello")
	triple := term.Triple{Subject: s, Predicate: p, Object: o}

	var count int
	for range store.Contexts(&triple) {
		count++
	}
	// No named graphs, so expect 0
	if count != 0 {
		t.Errorf("Contexts count = %d, want 0", count)
	}
}

func TestWithHTTPClient(t *testing.T) {
	_, store := newTestServer(t)
	_ = store

	// Verify WithHTTPClient option works
	custom := &http.Client{Timeout: 5 * time.Second}
	remote := sparqlstore.New("http://localhost:0/query",
		sparqlstore.WithHTTPClient(custom),
	)
	// The store should be created without error
	if remote == nil {
		t.Fatal("New returned nil with WithHTTPClient")
	}
}

func TestWithTimeout(t *testing.T) {
	remote := sparqlstore.New("http://localhost:0/query",
		sparqlstore.WithTimeout(10*time.Second),
	)
	if remote == nil {
		t.Fatal("New returned nil with WithTimeout")
	}
}

func TestAddWithNamedGraph(t *testing.T) {
	g := graph.NewGraph()
	ds := &sparql.Dataset{
		Default:     g,
		NamedGraphs: make(map[string]*rdflibgo.Graph),
	}
	srv := sparqlstore.NewServer(ds)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	graphURI := term.NewURIRefUnsafe("http://example.org/graph1")
	remote := sparqlstore.New(
		ts.URL+"/query",
		sparqlstore.WithUpdate(ts.URL+"/update"),
	)

	s := term.NewURIRefUnsafe("http://example.org/s")
	p := term.NewURIRefUnsafe("http://example.org/p")
	o := rdflibgo.NewLiteral("named")

	// Add with a URIRef context (named graph) — sends GRAPH <uri> { ... }
	remote.Add(term.Triple{Subject: s, Predicate: p, Object: o}, graphURI)

	// The named graph should exist in the server dataset after the update
	ng, ok := ds.NamedGraphs["http://example.org/graph1"]
	if !ok {
		t.Fatal("expected named graph to be created in dataset")
	}
	if ng.Len() != 1 {
		t.Errorf("named graph len = %d, want 1", ng.Len())
	}
}

func TestAddNoUpdateEndpoint(t *testing.T) {
	// Store without update endpoint — Add should be a no-op
	remote := sparqlstore.New("http://localhost:0/query")
	s := term.NewURIRefUnsafe("http://example.org/s")
	p := term.NewURIRefUnsafe("http://example.org/p")
	o := rdflibgo.NewLiteral("hello")

	// Should not panic
	remote.Add(term.Triple{Subject: s, Predicate: p, Object: o}, nil)
	remote.Remove(term.TriplePattern{Subject: s}, nil)
	remote.Set(term.Triple{Subject: s, Predicate: p, Object: o}, nil)
	remote.AddN(nil)
	remote.AddN([]term.Quad{{Triple: term.Triple{Subject: s, Predicate: p, Object: o}}})
}

func TestTriplesWithBNodeObjects(t *testing.T) {
	_, store := newTestServer(t)

	s := term.NewURIRefUnsafe("http://example.org/s")
	p := term.NewURIRefUnsafe("http://example.org/p")
	bnode := term.NewBNode()

	store.Add(term.Triple{Subject: s, Predicate: p, Object: bnode}, nil)

	var count int
	for tr := range store.Triples(term.TriplePattern{}, nil) {
		count++
		if _, ok := tr.Object.(term.BNode); !ok {
			t.Errorf("expected BNode object, got %T", tr.Object)
		}
	}
	if count != 1 {
		t.Errorf("expected 1 triple, got %d", count)
	}
}

func TestTriplesWithTypedLiteral(t *testing.T) {
	_, store := newTestServer(t)

	s := term.NewURIRefUnsafe("http://example.org/s")
	p := term.NewURIRefUnsafe("http://example.org/p")
	o := rdflibgo.NewLiteral(42)

	store.Add(term.Triple{Subject: s, Predicate: p, Object: o}, nil)

	var count int
	for tr := range store.Triples(term.TriplePattern{}, nil) {
		count++
		lit, ok := tr.Object.(term.Literal)
		if !ok {
			t.Fatalf("expected Literal, got %T", tr.Object)
		}
		if lit.Datatype() != term.XSDInteger {
			t.Errorf("datatype = %v, want xsd:integer", lit.Datatype())
		}
	}
	if count != 1 {
		t.Errorf("expected 1 triple, got %d", count)
	}
}

func TestTriplesWithLangLiteral(t *testing.T) {
	_, store := newTestServer(t)

	s := term.NewURIRefUnsafe("http://example.org/s")
	p := term.NewURIRefUnsafe("http://example.org/p")
	o := rdflibgo.NewLiteral("bonjour", rdflibgo.WithLang("fr"))

	store.Add(term.Triple{Subject: s, Predicate: p, Object: o}, nil)

	for tr := range store.Triples(term.TriplePattern{}, nil) {
		lit, ok := tr.Object.(term.Literal)
		if !ok {
			t.Fatalf("expected Literal, got %T", tr.Object)
		}
		if lit.Language() != "fr" {
			t.Errorf("language = %q, want %q", lit.Language(), "fr")
		}
	}
}

func TestTriplesErrorHandling(t *testing.T) {
	// Store pointing at a server that returns errors
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	t.Cleanup(ts.Close)

	remote := sparqlstore.New(ts.URL)
	var count int
	for range remote.Triples(term.TriplePattern{}, nil) {
		count++
	}
	if count != 0 {
		t.Errorf("expected 0 triples on error, got %d", count)
	}
}

func TestLenErrorHandling(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "error", http.StatusInternalServerError)
	}))
	t.Cleanup(ts.Close)

	remote := sparqlstore.New(ts.URL)
	if n := remote.Len(nil); n != 0 {
		t.Errorf("Len on error = %d, want 0", n)
	}
}

func TestContextsErrorHandling(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "error", http.StatusInternalServerError)
	}))
	t.Cleanup(ts.Close)

	remote := sparqlstore.New(ts.URL)
	var count int
	for range remote.Contexts(nil) {
		count++
	}
	if count != 0 {
		t.Errorf("Contexts on error = %d, want 0", count)
	}
}

func TestExecUpdateError(t *testing.T) {
	// Server returns 500 on update
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "update failed", http.StatusInternalServerError)
	}))
	t.Cleanup(ts.Close)

	remote := sparqlstore.New(ts.URL+"/query", sparqlstore.WithUpdate(ts.URL+"/update"))
	s := term.NewURIRefUnsafe("http://example.org/s")
	p := term.NewURIRefUnsafe("http://example.org/p")
	o := rdflibgo.NewLiteral("hello")

	// Should not panic even though update fails
	remote.Add(term.Triple{Subject: s, Predicate: p, Object: o}, nil)
}

func TestExecQueryJSONResponse(t *testing.T) {
	// Server returns JSON results
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/sparql-results+json")
		w.Write([]byte(`{"head":{"vars":["s","p","o"]},"results":{"bindings":[{"s":{"type":"uri","value":"http://example.org/x"},"p":{"type":"uri","value":"http://example.org/y"},"o":{"type":"literal","value":"z"}}]}}`))
	}))
	t.Cleanup(ts.Close)

	remote := sparqlstore.New(ts.URL)
	var count int
	for range remote.Triples(term.TriplePattern{}, nil) {
		count++
	}
	if count != 1 {
		t.Errorf("expected 1 triple from JSON, got %d", count)
	}
}

func TestLenWithNamedGraphContext(t *testing.T) {
	_, store := newTestServer(t)

	// Len with a URIRef context wraps in GRAPH — exercises wrapGraph with URIRef
	graphURI := term.NewURIRefUnsafe("http://example.org/graph1")
	n := store.Len(graphURI)
	if n != 0 {
		t.Errorf("Len(graph1) = %d, want 0", n)
	}
}

func TestRemoveWithWildcard(t *testing.T) {
	_, store := newTestServer(t)

	s := term.NewURIRefUnsafe("http://example.org/s")
	p := term.NewURIRefUnsafe("http://example.org/p")

	store.Add(term.Triple{Subject: s, Predicate: p, Object: rdflibgo.NewLiteral("a")}, nil)
	store.Add(term.Triple{Subject: s, Predicate: p, Object: rdflibgo.NewLiteral("b")}, nil)

	// Remove all triples with subject s (wildcard predicate and object)
	store.Remove(term.TriplePattern{Subject: s}, nil)

	if n := store.Len(nil); n != 0 {
		t.Errorf("Len after wildcard remove = %d, want 0", n)
	}
}

func TestExecQueryInvalidURL(t *testing.T) {
	// Invalid URL triggers NewRequestWithContext error
	remote := sparqlstore.New("://bad\x00url")

	var count int
	for range remote.Triples(term.TriplePattern{}, nil) {
		count++
	}
	if count != 0 {
		t.Errorf("expected 0 triples with bad URL, got %d", count)
	}
	if n := remote.Len(nil); n != 0 {
		t.Errorf("Len with bad URL = %d, want 0", n)
	}
	var ctxCount int
	for range remote.Contexts(nil) {
		ctxCount++
	}
	if ctxCount != 0 {
		t.Errorf("Contexts with bad URL = %d, want 0", ctxCount)
	}
}

func TestExecUpdateInvalidURL(t *testing.T) {
	// Invalid update URL triggers NewRequestWithContext error
	remote := sparqlstore.New("http://localhost:0/query",
		sparqlstore.WithUpdate("://bad\x00url"),
	)
	s := term.NewURIRefUnsafe("http://example.org/s")
	p := term.NewURIRefUnsafe("http://example.org/p")
	o := rdflibgo.NewLiteral("hello")

	// Should not panic
	remote.Add(term.Triple{Subject: s, Predicate: p, Object: o}, nil)
	remote.Set(term.Triple{Subject: s, Predicate: p, Object: o}, nil)
	remote.Remove(term.TriplePattern{Subject: s}, nil)
}

func TestExecQueryConnectionRefused(t *testing.T) {
	// Valid URL but nothing listening — client.Do error
	remote := sparqlstore.New("http://127.0.0.1:1/query")

	var count int
	for range remote.Triples(term.TriplePattern{}, nil) {
		count++
	}
	if count != 0 {
		t.Errorf("expected 0 triples on connection refused, got %d", count)
	}
}

func TestExecUpdateConnectionRefused(t *testing.T) {
	remote := sparqlstore.New("http://127.0.0.1:1/query",
		sparqlstore.WithUpdate("http://127.0.0.1:1/update"),
	)
	s := term.NewURIRefUnsafe("http://example.org/s")
	p := term.NewURIRefUnsafe("http://example.org/p")
	o := rdflibgo.NewLiteral("hello")

	// Should not panic
	remote.Add(term.Triple{Subject: s, Predicate: p, Object: o}, nil)
}

func TestTriplesWithObjectPattern(t *testing.T) {
	_, store := newTestServer(t)

	s := term.NewURIRefUnsafe("http://example.org/s")
	p := term.NewURIRefUnsafe("http://example.org/p")
	o1 := rdflibgo.NewLiteral("one")
	o2 := rdflibgo.NewLiteral("two")

	store.Add(term.Triple{Subject: s, Predicate: p, Object: o1}, nil)
	store.Add(term.Triple{Subject: s, Predicate: p, Object: o2}, nil)

	// Filter by all three positions
	var count int
	for range store.Triples(term.TriplePattern{Subject: s, Predicate: &p, Object: o1}, nil) {
		count++
	}
	if count != 1 {
		t.Errorf("triples with full pattern = %d, want 1", count)
	}
}

func TestTriplesEarlyBreak(t *testing.T) {
	_, store := newTestServer(t)

	s := term.NewURIRefUnsafe("http://example.org/s")
	p := term.NewURIRefUnsafe("http://example.org/p")
	store.Add(term.Triple{Subject: s, Predicate: p, Object: rdflibgo.NewLiteral("a")}, nil)
	store.Add(term.Triple{Subject: s, Predicate: p, Object: rdflibgo.NewLiteral("b")}, nil)
	store.Add(term.Triple{Subject: s, Predicate: p, Object: rdflibgo.NewLiteral("c")}, nil)

	// Break after first triple
	var count int
	for range store.Triples(term.TriplePattern{}, nil) {
		count++
		if count == 1 {
			break
		}
	}
	if count != 1 {
		t.Errorf("early break: got %d, want 1", count)
	}
}

func TestContextsEarlyBreak(t *testing.T) {
	// Mock server returning multiple graph URIs
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/sparql-results+xml")
		w.Write([]byte(`<?xml version="1.0"?>
<sparql xmlns="http://www.w3.org/2005/sparql-results#">
<head><variable name="g"/></head>
<results>
<result><binding name="g"><uri>http://example.org/g1</uri></binding></result>
<result><binding name="g"><uri>http://example.org/g2</uri></binding></result>
<result><binding name="g"><uri>http://example.org/g3</uri></binding></result>
</results>
</sparql>`))
	}))
	t.Cleanup(ts.Close)

	remote := sparqlstore.New(ts.URL)
	var count int
	for range remote.Contexts(nil) {
		count++
		if count == 1 {
			break
		}
	}
	if count != 1 {
		t.Errorf("early break in Contexts: got %d, want 1", count)
	}
}

func TestNamespacesEarlyBreak(t *testing.T) {
	_, store := newTestServer(t)

	store.Bind("ex", term.NewURIRefUnsafe("http://example.org/"))
	store.Bind("foaf", term.NewURIRefUnsafe("http://xmlns.com/foaf/0.1/"))
	store.Bind("dc", term.NewURIRefUnsafe("http://purl.org/dc/elements/1.1/"))

	var count int
	for range store.Namespaces() {
		count++
		if count == 1 {
			break
		}
	}
	if count != 1 {
		t.Errorf("early break in Namespaces: got %d, want 1", count)
	}
}

func TestTriplesMalformedResult(t *testing.T) {
	// Server returns results where subject is a literal (can't cast to term.Subject)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/sparql-results+xml")
		w.Write([]byte(`<?xml version="1.0"?>
<sparql xmlns="http://www.w3.org/2005/sparql-results#">
<head><variable name="s"/><variable name="p"/><variable name="o"/></head>
<results>
<result>
<binding name="s"><literal>not-a-subject</literal></binding>
<binding name="p"><uri>http://example.org/p</uri></binding>
<binding name="o"><literal>val</literal></binding>
</result>
</results>
</sparql>`))
	}))
	t.Cleanup(ts.Close)

	remote := sparqlstore.New(ts.URL)
	var count int
	for range remote.Triples(term.TriplePattern{}, nil) {
		count++
	}
	// Should skip the malformed row
	if count != 0 {
		t.Errorf("expected 0 triples from malformed result, got %d", count)
	}
}

func TestLenNonLiteralCount(t *testing.T) {
	// Server returns a non-literal for COUNT result
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/sparql-results+xml")
		w.Write([]byte(`<?xml version="1.0"?>
<sparql xmlns="http://www.w3.org/2005/sparql-results#">
<head><variable name="c"/></head>
<results>
<result>
<binding name="c"><uri>http://example.org/not-a-number</uri></binding>
</result>
</results>
</sparql>`))
	}))
	t.Cleanup(ts.Close)

	remote := sparqlstore.New(ts.URL)
	if n := remote.Len(nil); n != 0 {
		t.Errorf("Len with non-literal count = %d, want 0", n)
	}
}

func TestLenMissingCountVar(t *testing.T) {
	// Server returns a result without the "c" variable
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/sparql-results+xml")
		w.Write([]byte(`<?xml version="1.0"?>
<sparql xmlns="http://www.w3.org/2005/sparql-results#">
<head><variable name="x"/></head>
<results>
<result>
<binding name="x"><literal datatype="http://www.w3.org/2001/XMLSchema#integer">5</literal></binding>
</result>
</results>
</sparql>`))
	}))
	t.Cleanup(ts.Close)

	remote := sparqlstore.New(ts.URL)
	if n := remote.Len(nil); n != 0 {
		t.Errorf("Len with missing count var = %d, want 0", n)
	}
}

func TestAddNWithGraphContext(t *testing.T) {
	// Tests termKeyOrDefault with non-nil Graph context
	g := graph.NewGraph()
	ds := &sparql.Dataset{
		Default:     g,
		NamedGraphs: make(map[string]*rdflibgo.Graph),
	}
	srv := sparqlstore.NewServer(ds)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	remote := sparqlstore.New(
		ts.URL+"/query",
		sparqlstore.WithUpdate(ts.URL+"/update"),
	)

	graphURI := term.NewURIRefUnsafe("http://example.org/graph1")
	s := term.NewURIRefUnsafe("http://example.org/s")
	p := term.NewURIRefUnsafe("http://example.org/p")

	quads := []term.Quad{
		{Triple: term.Triple{Subject: s, Predicate: p, Object: rdflibgo.NewLiteral("a")}, Graph: graphURI},
		{Triple: term.Triple{Subject: s, Predicate: p, Object: rdflibgo.NewLiteral("b")}, Graph: graphURI},
	}
	remote.AddN(quads)

	// Verify named graph was created
	ng, ok := ds.NamedGraphs["http://example.org/graph1"]
	if !ok {
		t.Fatal("expected named graph to be created")
	}
	if ng.Len() != 2 {
		t.Errorf("named graph len = %d, want 2", ng.Len())
	}
}
