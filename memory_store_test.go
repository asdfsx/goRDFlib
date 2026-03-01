package rdflibgo

import "testing"

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
