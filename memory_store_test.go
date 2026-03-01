package rdflibgo

import (
	"fmt"
	"testing"
)

// Ported from: test/test_store/test_store_memorystore.py

func TestMemoryStoreAddAndLen(t *testing.T) {
	// Ported from: rdflib memory store add/len behavior
	s := NewMemoryStore()
	sub, _ := NewURIRef("http://example.org/s")
	pred, _ := NewURIRef("http://example.org/p")
	obj := NewLiteral("hello")
	s.Add(Triple{Subject: sub, Predicate: pred, Object: obj}, nil)
	if s.Len(nil) != 1 {
		t.Errorf("expected 1, got %d", s.Len(nil))
	}
}

func TestMemoryStoreDuplicateAdd(t *testing.T) {
	// Ported from: rdflib memory store dedup behavior
	s := NewMemoryStore()
	sub, _ := NewURIRef("http://example.org/s")
	pred, _ := NewURIRef("http://example.org/p")
	obj := NewLiteral("hello")
	s.Add(Triple{Subject: sub, Predicate: pred, Object: obj}, nil)
	s.Add(Triple{Subject: sub, Predicate: pred, Object: obj}, nil)
	if s.Len(nil) != 1 {
		t.Errorf("duplicate add should not increase count, got %d", s.Len(nil))
	}
}

func TestMemoryStoreRemove(t *testing.T) {
	// Ported from: rdflib memory store remove behavior
	s := NewMemoryStore()
	sub, _ := NewURIRef("http://example.org/s")
	pred, _ := NewURIRef("http://example.org/p")
	obj := NewLiteral("hello")
	s.Add(Triple{Subject: sub, Predicate: pred, Object: obj}, nil)
	s.Remove(TriplePattern{Subject: sub, Predicate: &pred, Object: obj}, nil)
	if s.Len(nil) != 0 {
		t.Errorf("expected 0 after remove, got %d", s.Len(nil))
	}
}

func TestMemoryStoreTriplesSubjectPattern(t *testing.T) {
	// Ported from: rdflib memory store pattern matching
	s := NewMemoryStore()
	s1, _ := NewURIRef("http://example.org/s1")
	s2, _ := NewURIRef("http://example.org/s2")
	p, _ := NewURIRef("http://example.org/p")
	o := NewLiteral("v")
	s.Add(Triple{Subject: s1, Predicate: p, Object: o}, nil)
	s.Add(Triple{Subject: s2, Predicate: p, Object: o}, nil)

	count := 0
	s.Triples(TriplePattern{Subject: s1}, nil)(func(Triple) bool {
		count++
		return true
	})
	if count != 1 {
		t.Errorf("expected 1 match for s1, got %d", count)
	}
}

func TestMemoryStoreTriplesPredPattern(t *testing.T) {
	s := NewMemoryStore()
	sub, _ := NewURIRef("http://example.org/s")
	p1, _ := NewURIRef("http://example.org/p1")
	p2, _ := NewURIRef("http://example.org/p2")
	o := NewLiteral("v")
	s.Add(Triple{Subject: sub, Predicate: p1, Object: o}, nil)
	s.Add(Triple{Subject: sub, Predicate: p2, Object: o}, nil)

	count := 0
	s.Triples(TriplePattern{Predicate: &p1}, nil)(func(Triple) bool {
		count++
		return true
	})
	if count != 1 {
		t.Errorf("expected 1 match for p1, got %d", count)
	}
}

func TestMemoryStoreTriplesAll(t *testing.T) {
	s := NewMemoryStore()
	sub, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	s.Add(Triple{Subject: sub, Predicate: p, Object: NewLiteral("a")}, nil)
	s.Add(Triple{Subject: sub, Predicate: p, Object: NewLiteral("b")}, nil)

	count := 0
	s.Triples(TriplePattern{}, nil)(func(Triple) bool {
		count++
		return true
	})
	if count != 2 {
		t.Errorf("expected 2, got %d", count)
	}
}

func TestMemoryStoreAddNConcurrent(t *testing.T) {
	// Verify AddN is atomic under concurrent access
	s := NewMemoryStore()
	sub, _ := NewURIRef("http://example.org/s")
	pred, _ := NewURIRef("http://example.org/p")

	var quads []Quad
	for i := 0; i < 100; i++ {
		quads = append(quads, Quad{Triple: Triple{
			Subject: sub, Predicate: pred, Object: NewLiteral(fmt.Sprintf("v%d", i)),
		}})
	}

	done := make(chan struct{})
	go func() {
		s.AddN(quads)
		close(done)
	}()
	// Concurrent read while AddN is in progress
	for i := 0; i < 10; i++ {
		_ = s.Len(nil)
	}
	<-done

	if s.Len(nil) != 100 {
		t.Errorf("expected 100 after AddN, got %d", s.Len(nil))
	}
}

func TestMemoryStoreRemoveConcurrent(t *testing.T) {
	s := NewMemoryStore()
	sub, _ := NewURIRef("http://example.org/s")
	pred, _ := NewURIRef("http://example.org/p")
	for i := 0; i < 50; i++ {
		s.Add(Triple{Subject: sub, Predicate: pred, Object: NewLiteral(fmt.Sprintf("v%d", i))}, nil)
	}

	done := make(chan struct{})
	go func() {
		s.Remove(TriplePattern{Subject: sub}, nil)
		close(done)
	}()
	for i := 0; i < 10; i++ {
		_ = s.Len(nil)
	}
	<-done

	if s.Len(nil) != 0 {
		t.Errorf("expected 0 after remove, got %d", s.Len(nil))
	}
}

func TestMemoryStoreObjectPattern(t *testing.T) {
	s := NewMemoryStore()
	s1, _ := NewURIRef("http://example.org/s1")
	s2, _ := NewURIRef("http://example.org/s2")
	p, _ := NewURIRef("http://example.org/p")
	o := NewLiteral("v")
	s.Add(Triple{Subject: s1, Predicate: p, Object: o}, nil)
	s.Add(Triple{Subject: s2, Predicate: p, Object: o}, nil)

	count := 0
	s.Triples(TriplePattern{Object: o}, nil)(func(Triple) bool {
		count++
		return true
	})
	if count != 2 {
		t.Errorf("expected 2 match for object pattern, got %d", count)
	}
}

func TestMemoryStoreExactPattern(t *testing.T) {
	s := NewMemoryStore()
	sub, _ := NewURIRef("http://example.org/s")
	pred, _ := NewURIRef("http://example.org/p")
	obj := NewLiteral("v")
	s.Add(Triple{Subject: sub, Predicate: pred, Object: obj}, nil)

	count := 0
	s.Triples(TriplePattern{Subject: sub, Predicate: &pred, Object: obj}, nil)(func(Triple) bool {
		count++
		return true
	})
	if count != 1 {
		t.Errorf("expected 1, got %d", count)
	}
}

func TestMemoryStoreNamespaces(t *testing.T) {
	// Ported from: rdflib memory store namespace binding
	s := NewMemoryStore()
	ns, _ := NewURIRef("http://example.org/ns#")
	s.Bind("ex", ns)

	got, ok := s.Namespace("ex")
	if !ok || got != ns {
		t.Errorf("namespace lookup failed")
	}
	prefix, ok := s.Prefix(ns)
	if !ok || prefix != "ex" {
		t.Errorf("prefix lookup failed")
	}
}

func TestMemoryStoreContexts(t *testing.T) {
	s := NewMemoryStore()
	count := 0
	s.Contexts(nil)(func(Term) bool {
		count++
		return true
	})
	if count != 0 {
		t.Errorf("expected 0 contexts, got %d", count)
	}
}

// --- Benchmarks ---

func BenchmarkMemoryStoreAdd(b *testing.B) {
	s := NewMemoryStore()
	pred, _ := NewURIRef("http://example.org/p")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sub := NewURIRefUnsafe(fmt.Sprintf("http://example.org/s%d", i))
		s.Add(Triple{Subject: sub, Predicate: pred, Object: NewLiteral(i)}, nil)
	}
}

func BenchmarkMemoryStoreTriples(b *testing.B) {
	s := NewMemoryStore()
	sub, _ := NewURIRef("http://example.org/s")
	pred, _ := NewURIRef("http://example.org/p")
	for i := 0; i < 1000; i++ {
		s.Add(Triple{Subject: sub, Predicate: pred, Object: NewLiteral(i)}, nil)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Triples(TriplePattern{Subject: sub}, nil)(func(Triple) bool { return true })
	}
}
