package rdflibgo

import "testing"

// Ported from: test/test_graph/test_graph.py

func makeTestGraph(t *testing.T) (*Graph, URIRef, URIRef, Literal) {
	t.Helper()
	g := NewGraph()
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	o := NewLiteral("hello")
	g.Add(s, p, o)
	return g, s, p, o
}

func TestGraphAddAndLen(t *testing.T) {
	// Ported from: rdflib.graph.Graph.add, __len__
	g, _, _, _ := makeTestGraph(t)
	if g.Len() != 1 {
		t.Errorf("expected 1, got %d", g.Len())
	}
}

func TestGraphContains(t *testing.T) {
	// Ported from: rdflib.graph.Graph.__contains__
	g, s, p, o := makeTestGraph(t)
	if !g.Contains(s, p, o) {
		t.Error("should contain the triple")
	}
	p2, _ := NewURIRef("http://example.org/other")
	if g.Contains(s, p2, o) {
		t.Error("should not contain different predicate")
	}
}

func TestGraphRemove(t *testing.T) {
	// Ported from: rdflib.graph.Graph.remove
	g, s, p, o := makeTestGraph(t)
	g.Remove(s, &p, o)
	if g.Len() != 0 {
		t.Errorf("expected 0 after remove, got %d", g.Len())
	}
}

func TestGraphSet(t *testing.T) {
	// Ported from: rdflib.graph.Graph.set
	g, s, p, _ := makeTestGraph(t)
	newObj := NewLiteral("world")
	g.Set(s, p, newObj)
	if g.Len() != 1 {
		t.Errorf("expected 1 after set, got %d", g.Len())
	}
	val, ok := g.Value(s, &p, nil)
	if !ok {
		t.Fatal("expected a value")
	}
	if val.N3() != newObj.N3() {
		t.Errorf("expected %q, got %q", newObj.N3(), val.N3())
	}
}

func TestGraphTriples(t *testing.T) {
	// Ported from: rdflib.graph.Graph.triples
	g, s, _, _ := makeTestGraph(t)
	p2, _ := NewURIRef("http://example.org/p2")
	g.Add(s, p2, NewLiteral("world"))

	count := 0
	g.Triples(s, nil, nil)(func(Triple) bool {
		count++
		return true
	})
	if count != 2 {
		t.Errorf("expected 2, got %d", count)
	}
}

func TestGraphSubjects(t *testing.T) {
	// Ported from: rdflib.graph.Graph.subjects
	g := NewGraph()
	s1, _ := NewURIRef("http://example.org/s1")
	s2, _ := NewURIRef("http://example.org/s2")
	p, _ := NewURIRef("http://example.org/p")
	o := NewLiteral("v")
	g.Add(s1, p, o)
	g.Add(s2, p, o)

	var subjects []Term
	g.Subjects(&p, o)(func(t Term) bool {
		subjects = append(subjects, t)
		return true
	})
	if len(subjects) != 2 {
		t.Errorf("expected 2 subjects, got %d", len(subjects))
	}
}

func TestGraphObjects(t *testing.T) {
	// Ported from: rdflib.graph.Graph.objects
	g := NewGraph()
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	g.Add(s, p, NewLiteral("a"))
	g.Add(s, p, NewLiteral("b"))

	var objects []Term
	g.Objects(s, &p)(func(t Term) bool {
		objects = append(objects, t)
		return true
	})
	if len(objects) != 2 {
		t.Errorf("expected 2 objects, got %d", len(objects))
	}
}

func TestGraphValue(t *testing.T) {
	// Ported from: rdflib.graph.Graph.value
	g, s, p, o := makeTestGraph(t)
	val, ok := g.Value(s, &p, nil)
	if !ok || val.N3() != o.N3() {
		t.Errorf("expected %q, got %v", o.N3(), val)
	}
}

func TestGraphFluent(t *testing.T) {
	// Test fluent (chaining) API
	g := NewGraph()
	s, _ := NewURIRef("http://example.org/s")
	p1, _ := NewURIRef("http://example.org/p1")
	p2, _ := NewURIRef("http://example.org/p2")
	g.Add(s, p1, NewLiteral("a")).Add(s, p2, NewLiteral("b"))
	if g.Len() != 2 {
		t.Errorf("expected 2, got %d", g.Len())
	}
}

func TestGraphDefaultNamespaces(t *testing.T) {
	// Ported from: rdflib.graph.Graph default namespace bindings
	g := NewGraph()
	count := 0
	g.Namespaces()(func(prefix string, ns URIRef) bool {
		count++
		return true
	})
	if count < 4 {
		t.Errorf("expected at least 4 default namespaces, got %d", count)
	}
}

func TestGraphQName(t *testing.T) {
	// Ported from: rdflib.graph.Graph.qname
	g := NewGraph()
	got := g.QName("http://www.w3.org/2001/XMLSchema#string")
	if got != "xsd:string" {
		t.Errorf("expected xsd:string, got %q", got)
	}
}

func TestGraphBind(t *testing.T) {
	// Ported from: rdflib.graph.Graph.bind
	g := NewGraph()
	ns, _ := NewURIRef("http://example.org/ns#")
	g.Bind("ex", ns)
	got := g.QName("http://example.org/ns#Thing")
	if got != "ex:Thing" {
		t.Errorf("expected ex:Thing, got %q", got)
	}
}

func TestGraphUnion(t *testing.T) {
	// Ported from: rdflib.graph.Graph.__iadd__
	g1 := NewGraph()
	g2 := NewGraph()
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	g1.Add(s, p, NewLiteral("a"))
	g2.Add(s, p, NewLiteral("b"))

	u := g1.Union(g2)
	if u.Len() != 2 {
		t.Errorf("expected 2, got %d", u.Len())
	}
}

func TestGraphIntersection(t *testing.T) {
	// Ported from: rdflib.graph.Graph.__mul__
	g1 := NewGraph()
	g2 := NewGraph()
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	shared := NewLiteral("shared")
	g1.Add(s, p, shared)
	g1.Add(s, p, NewLiteral("only1"))
	g2.Add(s, p, shared)
	g2.Add(s, p, NewLiteral("only2"))

	inter := g1.Intersection(g2)
	if inter.Len() != 1 {
		t.Errorf("expected 1, got %d", inter.Len())
	}
}

func TestGraphDifference(t *testing.T) {
	// Ported from: rdflib.graph.Graph.__isub__
	g1 := NewGraph()
	g2 := NewGraph()
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	shared := NewLiteral("shared")
	g1.Add(s, p, shared)
	g1.Add(s, p, NewLiteral("only1"))
	g2.Add(s, p, shared)

	diff := g1.Difference(g2)
	if diff.Len() != 1 {
		t.Errorf("expected 1, got %d", diff.Len())
	}
}

func TestGraphConnected(t *testing.T) {
	// Ported from: rdflib.graph.Graph.connected
	g := NewGraph()
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	o, _ := NewURIRef("http://example.org/o")
	g.Add(s, p, o)

	if !g.Connected() {
		t.Error("single-edge graph should be connected")
	}

	// Disconnected graph
	g2 := NewGraph()
	s2, _ := NewURIRef("http://example.org/s2")
	o2, _ := NewURIRef("http://example.org/o2")
	g2.Add(s, p, o)
	g2.Add(s2, p, o2)
	if g2.Connected() {
		t.Error("disconnected graph should not be connected")
	}
}

func TestGraphAllNodes(t *testing.T) {
	// Ported from: rdflib.graph.Graph.all_nodes
	g := NewGraph()
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	o, _ := NewURIRef("http://example.org/o")
	g.Add(s, p, o)

	nodes := g.AllNodes()
	if len(nodes) != 2 {
		t.Errorf("expected 2 nodes (s and o, not p), got %d", len(nodes))
	}
}
