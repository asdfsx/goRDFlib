package badgerstore

import (
	"sync"
	"testing"

	"github.com/tggo/goRDFlib/term"
)

func newTestStore(t *testing.T) *BadgerStore {
	t.Helper()
	s, err := New(WithInMemory())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

var (
	alice  = term.NewURIRefUnsafe("http://example.org/Alice")
	bob    = term.NewURIRefUnsafe("http://example.org/Bob")
	name   = term.NewURIRefUnsafe("http://example.org/name")
	age    = term.NewURIRefUnsafe("http://example.org/age")
	knows  = term.NewURIRefUnsafe("http://example.org/knows")
	rdfT   = term.NewURIRefUnsafe("http://www.w3.org/1999/02/22-rdf-syntax-ns#type")
	foafP  = term.NewURIRefUnsafe("http://xmlns.com/foaf/0.1/Person")
	graph1 = term.NewURIRefUnsafe("http://example.org/graph1")
	graph2 = term.NewURIRefUnsafe("http://example.org/graph2")
)

func TestAddAndLen(t *testing.T) {
	s := newTestStore(t)
	s.Add(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}, nil)
	s.Add(term.Triple{Subject: alice, Predicate: age, Object: term.NewLiteral(30)}, nil)
	if got := s.Len(nil); got != 2 {
		t.Errorf("Len = %d, want 2", got)
	}
}

func TestDuplicateAdd(t *testing.T) {
	s := newTestStore(t)
	triple := term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}
	s.Add(triple, nil)
	s.Add(triple, nil) // duplicate
	if got := s.Len(nil); got != 1 {
		t.Errorf("Len after duplicate = %d, want 1", got)
	}
}

func TestRemove(t *testing.T) {
	s := newTestStore(t)
	t1 := term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}
	t2 := term.Triple{Subject: alice, Predicate: age, Object: term.NewLiteral(30)}
	s.Add(t1, nil)
	s.Add(t2, nil)

	// Remove by exact pattern.
	s.Remove(term.TriplePattern{Subject: alice, Predicate: &name, Object: term.NewLiteral("Alice")}, nil)
	if got := s.Len(nil); got != 1 {
		t.Errorf("Len after remove = %d, want 1", got)
	}
}

func TestRemoveWildcard(t *testing.T) {
	s := newTestStore(t)
	s.Add(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}, nil)
	s.Add(term.Triple{Subject: alice, Predicate: age, Object: term.NewLiteral(30)}, nil)
	s.Add(term.Triple{Subject: bob, Predicate: name, Object: term.NewLiteral("Bob")}, nil)

	// Remove all triples with subject alice.
	s.Remove(term.TriplePattern{Subject: alice}, nil)
	if got := s.Len(nil); got != 1 {
		t.Errorf("Len after wildcard remove = %d, want 1", got)
	}
}

func TestSet(t *testing.T) {
	s := newTestStore(t)
	s.Add(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}, nil)
	s.Set(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice B.")}, nil)

	if got := s.Len(nil); got != 1 {
		t.Errorf("Len after Set = %d, want 1", got)
	}

	// Verify the new value.
	count := 0
	for tr := range s.Triples(term.TriplePattern{Subject: alice, Predicate: &name}, nil) {
		count++
		if lit, ok := tr.Object.(term.Literal); ok {
			if lit.Lexical() != "Alice B." {
				t.Errorf("Set value = %q, want %q", lit.Lexical(), "Alice B.")
			}
		} else {
			t.Error("object is not Literal")
		}
	}
	if count != 1 {
		t.Errorf("Triples count = %d, want 1", count)
	}
}

func TestTriplesAllPatterns(t *testing.T) {
	s := newTestStore(t)
	s.Add(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}, nil)
	s.Add(term.Triple{Subject: alice, Predicate: knows, Object: bob}, nil)
	s.Add(term.Triple{Subject: bob, Predicate: name, Object: term.NewLiteral("Bob")}, nil)

	tests := []struct {
		name    string
		pattern term.TriplePattern
		want    int
	}{
		{"all", term.TriplePattern{}, 3},
		{"s", term.TriplePattern{Subject: alice}, 2},
		{"p", term.TriplePattern{Predicate: &name}, 2},
		{"o", term.TriplePattern{Object: bob}, 1},
		{"sp", term.TriplePattern{Subject: alice, Predicate: &name}, 1},
		{"so", term.TriplePattern{Subject: alice, Object: bob}, 1},
		{"po", term.TriplePattern{Predicate: &name, Object: term.NewLiteral("Alice")}, 1},
		{"spo", term.TriplePattern{Subject: alice, Predicate: &name, Object: term.NewLiteral("Alice")}, 1},
		{"no match", term.TriplePattern{Subject: bob, Predicate: &knows}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count := 0
			for range s.Triples(tt.pattern, nil) {
				count++
			}
			if count != tt.want {
				t.Errorf("got %d, want %d", count, tt.want)
			}
		})
	}
}

func TestAddN(t *testing.T) {
	s := newTestStore(t)
	quads := []term.Quad{
		{Triple: term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}},
		{Triple: term.Triple{Subject: bob, Predicate: name, Object: term.NewLiteral("Bob")}},
		{Triple: term.Triple{Subject: alice, Predicate: knows, Object: bob}},
	}
	s.AddN(quads)
	if got := s.Len(nil); got != 3 {
		t.Errorf("Len = %d, want 3", got)
	}
}

func TestNamespaceBindings(t *testing.T) {
	s := newTestStore(t)
	ex := term.NewURIRefUnsafe("http://example.org/")
	foaf := term.NewURIRefUnsafe("http://xmlns.com/foaf/0.1/")

	s.Bind("ex", ex)
	s.Bind("foaf", foaf)

	ns, ok := s.Namespace("ex")
	if !ok || ns != ex {
		t.Errorf("Namespace(ex) = %v, %v", ns, ok)
	}

	prefix, ok := s.Prefix(foaf)
	if !ok || prefix != "foaf" {
		t.Errorf("Prefix(foaf) = %q, %v", prefix, ok)
	}

	_, ok = s.Namespace("nonexistent")
	if ok {
		t.Error("Namespace(nonexistent) should be false")
	}

	count := 0
	for range s.Namespaces() {
		count++
	}
	if count != 2 {
		t.Errorf("Namespaces count = %d, want 2", count)
	}
}

func TestNamedGraphs(t *testing.T) {
	s := newTestStore(t)
	t1 := term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}
	t2 := term.Triple{Subject: bob, Predicate: name, Object: term.NewLiteral("Bob")}

	s.Add(t1, graph1)
	s.Add(t2, graph2)
	s.Add(t1, nil) // default graph

	if got := s.Len(graph1); got != 1 {
		t.Errorf("Len(graph1) = %d, want 1", got)
	}
	if got := s.Len(graph2); got != 1 {
		t.Errorf("Len(graph2) = %d, want 1", got)
	}
	if got := s.Len(nil); got != 1 {
		t.Errorf("Len(default) = %d, want 1", got)
	}
}

func TestContexts(t *testing.T) {
	s := newTestStore(t)
	s.Add(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}, graph1)
	s.Add(term.Triple{Subject: bob, Predicate: name, Object: term.NewLiteral("Bob")}, graph2)

	count := 0
	for range s.Contexts(nil) {
		count++
	}
	if count != 2 {
		t.Errorf("Contexts count = %d, want 2", count)
	}
}

func TestContextsFilteredByTriple(t *testing.T) {
	s := newTestStore(t)
	t1 := term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}
	s.Add(t1, graph1)
	s.Add(term.Triple{Subject: bob, Predicate: name, Object: term.NewLiteral("Bob")}, graph2)

	count := 0
	for range s.Contexts(&t1) {
		count++
	}
	if count != 1 {
		t.Errorf("Contexts(alice triple) count = %d, want 1", count)
	}
}

func TestContextAwareAndTransactionAware(t *testing.T) {
	s := newTestStore(t)
	if !s.ContextAware() {
		t.Error("ContextAware should be true")
	}
	if !s.TransactionAware() {
		t.Error("TransactionAware should be true")
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	s, err := New(WithDir(dir))
	if err != nil {
		t.Fatal(err)
	}
	s.Add(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}, nil)
	s.Bind("ex", term.NewURIRefUnsafe("http://example.org/"))
	s.Close()

	// Reopen.
	s2, err := New(WithDir(dir))
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()

	if got := s2.Len(nil); got != 1 {
		t.Errorf("Len after reopen = %d, want 1", got)
	}

	ns, ok := s2.Namespace("ex")
	if !ok || ns.Value() != "http://example.org/" {
		t.Errorf("Namespace after reopen = %v, %v", ns, ok)
	}

	count := 0
	for range s2.Triples(term.TriplePattern{Subject: alice}, nil) {
		count++
	}
	if count != 1 {
		t.Errorf("Triples after reopen = %d, want 1", count)
	}
}

func TestLiteralTypes(t *testing.T) {
	s := newTestStore(t)

	// String literal
	s.Add(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}, nil)
	// Integer literal
	s.Add(term.Triple{Subject: alice, Predicate: age, Object: term.NewLiteral(30)}, nil)
	// Language-tagged literal
	langLit := term.NewLiteral("Alice", term.WithLang("en"))
	s.Add(term.Triple{Subject: alice, Predicate: term.NewURIRefUnsafe("http://example.org/label"), Object: langLit}, nil)
	// Directional lang literal
	dirLit := term.NewLiteral("Alice", term.WithLang("en"), term.WithDir("ltr"))
	s.Add(term.Triple{Subject: alice, Predicate: term.NewURIRefUnsafe("http://example.org/dirLabel"), Object: dirLit}, nil)

	if got := s.Len(nil); got != 4 {
		t.Errorf("Len = %d, want 4", got)
	}

	// Verify round-trip of language-tagged literal.
	for tr := range s.Triples(term.TriplePattern{Subject: alice, Predicate: ptrURI("http://example.org/label")}, nil) {
		lit, ok := tr.Object.(term.Literal)
		if !ok {
			t.Fatal("expected Literal")
		}
		if lit.Language() != "en" {
			t.Errorf("Language = %q, want %q", lit.Language(), "en")
		}
	}

	// Verify round-trip of directional literal.
	for tr := range s.Triples(term.TriplePattern{Subject: alice, Predicate: ptrURI("http://example.org/dirLabel")}, nil) {
		lit, ok := tr.Object.(term.Literal)
		if !ok {
			t.Fatal("expected Literal")
		}
		if lit.Dir() != "ltr" {
			t.Errorf("Dir = %q, want %q", lit.Dir(), "ltr")
		}
	}
}

func ptrURI(s string) *term.URIRef {
	u := term.NewURIRefUnsafe(s)
	return &u
}

func TestBNodeTerms(t *testing.T) {
	s := newTestStore(t)
	bn := term.NewBNode("")
	s.Add(term.Triple{Subject: bn, Predicate: name, Object: term.NewLiteral("anon")}, nil)
	if got := s.Len(nil); got != 1 {
		t.Errorf("Len = %d, want 1", got)
	}

	count := 0
	for range s.Triples(term.TriplePattern{}, nil) {
		count++
	}
	if count != 1 {
		t.Errorf("Triples count = %d, want 1", count)
	}
}

func TestConcurrency(t *testing.T) {
	s := newTestStore(t)
	var wg sync.WaitGroup
	n := 100

	// Concurrent writes.
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			subj := term.NewURIRefUnsafe("http://example.org/" + string(rune('A'+i%26)))
			s.Add(term.Triple{Subject: subj, Predicate: name, Object: term.NewLiteral(i)}, nil)
		}(i)
	}
	wg.Wait()

	// Concurrent reads.
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range s.Triples(term.TriplePattern{}, nil) {
			}
		}()
	}
	wg.Wait()

	if got := s.Len(nil); got == 0 {
		t.Error("expected some triples after concurrent writes")
	}
}

func TestEarlyBreak(t *testing.T) {
	s := newTestStore(t)
	for i := 0; i < 10; i++ {
		subj := term.NewURIRefUnsafe("http://example.org/" + string(rune('A'+i)))
		s.Add(term.Triple{Subject: subj, Predicate: name, Object: term.NewLiteral(i)}, nil)
	}

	count := 0
	for range s.Triples(term.TriplePattern{}, nil) {
		count++
		if count == 3 {
			break
		}
	}
	if count != 3 {
		t.Errorf("early break count = %d, want 3", count)
	}
}

func TestPluginRegistration(t *testing.T) {
	// Verify the init() registered the store.
	// Import side-effect is enough; the plugin system is global.
	// Just verify the store compiles and the interface is satisfied.
	var _ interface {
		Add(term.Triple, term.Term)
		Len(term.Term) int
	} = newTestStore(t)
}

func TestRemoveFromNamedGraph(t *testing.T) {
	s := newTestStore(t)
	t1 := term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}
	s.Add(t1, graph1)
	s.Add(t1, nil) // also in default graph

	s.Remove(term.TriplePattern{Subject: alice}, graph1)
	if got := s.Len(graph1); got != 0 {
		t.Errorf("Len(graph1) after remove = %d, want 0", got)
	}
	// Default graph should still have it.
	if got := s.Len(nil); got != 1 {
		t.Errorf("Len(default) after remove = %d, want 1", got)
	}
}

func TestBNodeContextIgnored(t *testing.T) {
	s := newTestStore(t)
	bn := term.NewBNode("")
	t1 := term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}
	s.Add(t1, bn)

	// BNode context should be treated as default graph.
	if got := s.Len(nil); got != 1 {
		t.Errorf("Len(nil) with BNode ctx = %d, want 1", got)
	}
}

func TestEmptyStoreOperations(t *testing.T) {
	s := newTestStore(t)
	if got := s.Len(nil); got != 0 {
		t.Errorf("Len of empty store = %d, want 0", got)
	}

	count := 0
	for range s.Triples(term.TriplePattern{}, nil) {
		count++
	}
	if count != 0 {
		t.Errorf("Triples of empty store = %d, want 0", count)
	}

	count = 0
	for range s.Contexts(nil) {
		count++
	}
	if count != 0 {
		t.Errorf("Contexts of empty store = %d, want 0", count)
	}

	// Remove on empty store should not panic.
	s.Remove(term.TriplePattern{}, nil)
}

func TestAddNWithNamedGraphs(t *testing.T) {
	s := newTestStore(t)
	quads := []term.Quad{
		{Triple: term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}, Graph: graph1},
		{Triple: term.Triple{Subject: bob, Predicate: name, Object: term.NewLiteral("Bob")}, Graph: graph2},
	}
	s.AddN(quads)

	if got := s.Len(graph1); got != 1 {
		t.Errorf("Len(graph1) = %d, want 1", got)
	}
	if got := s.Len(graph2); got != 1 {
		t.Errorf("Len(graph2) = %d, want 1", got)
	}
}
