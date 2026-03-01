package rdflibgo

import "testing"

// Ported from: test/test_path.py

func makePathGraph(t *testing.T) *Graph {
	t.Helper()
	g := NewGraph()
	a, _ := NewURIRef("http://example.org/a")
	b, _ := NewURIRef("http://example.org/b")
	c, _ := NewURIRef("http://example.org/c")
	d, _ := NewURIRef("http://example.org/d")
	p, _ := NewURIRef("http://example.org/p")
	q, _ := NewURIRef("http://example.org/q")
	g.Add(a, p, b)
	g.Add(b, p, c)
	g.Add(c, p, d)
	g.Add(a, q, c)
	return g
}

func collectPairs(g *Graph, path Path, subj Subject, obj Term) [][2]string {
	var result [][2]string
	path.Eval(g, subj, obj)(func(s, o Term) bool {
		result = append(result, [2]string{s.N3(), o.N3()})
		return true
	})
	return result
}

func TestURIRefPath(t *testing.T) {
	// Ported from: rdflib.paths — basic URIRef path evaluation
	g := makePathGraph(t)
	a, _ := NewURIRef("http://example.org/a")
	p, _ := NewURIRef("http://example.org/p")

	pairs := collectPairs(g, AsPath(p), a, nil)
	if len(pairs) != 1 || pairs[0][1] != "<http://example.org/b>" {
		t.Errorf("expected [(a,b)], got %v", pairs)
	}
}

func TestInvPath(t *testing.T) {
	// Ported from: rdflib.paths.InvPath — inverse traversal ^p
	g := makePathGraph(t)
	b, _ := NewURIRef("http://example.org/b")
	p, _ := NewURIRef("http://example.org/p")

	pairs := collectPairs(g, Inv(AsPath(p)), b, nil)
	if len(pairs) != 1 || pairs[0][1] != "<http://example.org/a>" {
		t.Errorf("expected [(b,a)], got %v", pairs)
	}
}

func TestSequencePath(t *testing.T) {
	// Ported from: rdflib.paths.SequencePath — p/p
	g := makePathGraph(t)
	a, _ := NewURIRef("http://example.org/a")
	p, _ := NewURIRef("http://example.org/p")

	path := Sequence(AsPath(p), AsPath(p))
	pairs := collectPairs(g, path, a, nil)
	if len(pairs) != 1 || pairs[0][1] != "<http://example.org/c>" {
		t.Errorf("expected [(a,c)], got %v", pairs)
	}
}

func TestSequencePathTriple(t *testing.T) {
	// p/p/p from a → d
	g := makePathGraph(t)
	a, _ := NewURIRef("http://example.org/a")
	p, _ := NewURIRef("http://example.org/p")

	path := Sequence(AsPath(p), AsPath(p), AsPath(p))
	pairs := collectPairs(g, path, a, nil)
	if len(pairs) != 1 || pairs[0][1] != "<http://example.org/d>" {
		t.Errorf("expected [(a,d)], got %v", pairs)
	}
}

func TestAlternativePath(t *testing.T) {
	// Ported from: rdflib.paths.AlternativePath — p|q
	g := makePathGraph(t)
	a, _ := NewURIRef("http://example.org/a")
	p, _ := NewURIRef("http://example.org/p")
	q, _ := NewURIRef("http://example.org/q")

	path := Alternative(AsPath(p), AsPath(q))
	pairs := collectPairs(g, path, a, nil)
	if len(pairs) != 2 {
		t.Errorf("expected 2 pairs, got %d: %v", len(pairs), pairs)
	}
}

func TestZeroOrMorePath(t *testing.T) {
	// Ported from: rdflib.paths.MulPath p* — transitive closure with identity
	g := makePathGraph(t)
	a, _ := NewURIRef("http://example.org/a")
	p, _ := NewURIRef("http://example.org/p")

	path := ZeroOrMore(AsPath(p))
	pairs := collectPairs(g, path, a, nil)
	// a→a (zero), a→b, a→c, a→d
	if len(pairs) != 4 {
		t.Errorf("expected 4 pairs for p*, got %d: %v", len(pairs), pairs)
	}
}

func TestOneOrMorePath(t *testing.T) {
	// Ported from: rdflib.paths.MulPath p+ — transitive closure without identity
	g := makePathGraph(t)
	a, _ := NewURIRef("http://example.org/a")
	p, _ := NewURIRef("http://example.org/p")

	path := OneOrMore(AsPath(p))
	pairs := collectPairs(g, path, a, nil)
	// a→b, a→c, a→d (no identity)
	if len(pairs) != 3 {
		t.Errorf("expected 3 pairs for p+, got %d: %v", len(pairs), pairs)
	}
}

func TestZeroOrOnePath(t *testing.T) {
	// Ported from: rdflib.paths.MulPath p? — single step or identity
	g := makePathGraph(t)
	a, _ := NewURIRef("http://example.org/a")
	p, _ := NewURIRef("http://example.org/p")

	path := ZeroOrOne(AsPath(p))
	pairs := collectPairs(g, path, a, nil)
	// a→a (zero), a→b (one step)
	if len(pairs) != 2 {
		t.Errorf("expected 2 pairs for p?, got %d: %v", len(pairs), pairs)
	}
}

func TestNegatedPath(t *testing.T) {
	// Ported from: rdflib.paths.NegatedPath — !p excludes predicate
	g := makePathGraph(t)
	a, _ := NewURIRef("http://example.org/a")
	p, _ := NewURIRef("http://example.org/p")

	path := Negated(p)
	pairs := collectPairs(g, path, a, nil)
	// a has p→b and q→c; negating p should only return q→c
	if len(pairs) != 1 || pairs[0][1] != "<http://example.org/c>" {
		t.Errorf("expected [(a,c)] via !p, got %v", pairs)
	}
}

func TestPathCycleDetection(t *testing.T) {
	// Ported from: rdflib.paths — cycle handling in p*
	g := NewGraph()
	a, _ := NewURIRef("http://example.org/a")
	b, _ := NewURIRef("http://example.org/b")
	p, _ := NewURIRef("http://example.org/p")
	g.Add(a, p, b)
	g.Add(b, p, a) // cycle!

	path := ZeroOrMore(AsPath(p))
	pairs := collectPairs(g, path, a, nil)
	// a→a (zero), a→b (one step), b→a (via b→a) — should not infinite loop
	if len(pairs) != 3 {
		t.Errorf("expected 3 pairs with cycle, got %d: %v", len(pairs), pairs)
	}
}

func TestPathDSL(t *testing.T) {
	// Test builder API: Slash and Or
	p, _ := NewURIRef("http://example.org/p")
	q, _ := NewURIRef("http://example.org/q")

	seq := AsPath(p).Slash(AsPath(q))
	if len(seq.Args) != 2 {
		t.Errorf("expected 2 args in sequence")
	}

	alt := AsPath(p).Or(AsPath(q))
	if len(alt.Args) != 2 {
		t.Errorf("expected 2 args in alternative")
	}
}
