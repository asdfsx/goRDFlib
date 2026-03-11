package reasoning

import (
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/namespace"
	"github.com/tggo/goRDFlib/term"
)

// --- Phase 2: Extended property rules ---

// --- prp-spo1: property chain axiom ---

func TestOWLRL_PropertyChainAxiom(t *testing.T) {
	g := graph.NewGraph()
	uncle := ex.Term("uncle")
	parent := ex.Term("parent")
	brother := ex.Term("brother")

	// uncle = parent . brother
	chain := buildRDFList(g, []term.Term{parent, brother})
	g.Add(uncle, namespace.OWL.PropertyChainAxiom, chain)

	g.Add(exA, parent, exB)
	g.Add(exB, brother, exC)

	OWLRLClosure(g)

	if !g.Contains(exA, uncle, exC) {
		t.Error("expected A uncle C via property chain (parent . brother)")
	}
}

func TestOWLRL_PropertyChainAxiomThreeSteps(t *testing.T) {
	g := graph.NewGraph()
	result := ex.Term("result")
	p1 := ex.Term("p1")
	p2 := ex.Term("p2")
	p3 := ex.Term("p3")

	chain := buildRDFList(g, []term.Term{p1, p2, p3})
	g.Add(result, namespace.OWL.PropertyChainAxiom, chain)

	g.Add(exA, p1, exB)
	g.Add(exB, p2, exC)
	g.Add(exC, p3, exD)

	OWLRLClosure(g)

	if !g.Contains(exA, result, exD) {
		t.Error("expected A result D via 3-step chain")
	}
}

// --- prp-key: hasKey ---

func TestOWLRL_HasKey(t *testing.T) {
	g := graph.NewGraph()
	keyProp := ex.Term("ssn")
	cls := ex.Term("Person")

	keys := buildRDFList(g, []term.Term{keyProp})
	g.Add(cls, namespace.OWL.HasKey, keys)

	g.Add(exA, namespace.RDF.Type, cls)
	g.Add(exB, namespace.RDF.Type, cls)
	ssn := term.NewLiteral("123-45-6789")
	g.Add(exA, keyProp, ssn)
	g.Add(exB, keyProp, ssn)

	OWLRLClosure(g)

	hasSameAs := g.Contains(exA, namespace.OWL.SameAs, exB) ||
		g.Contains(exB, namespace.OWL.SameAs, exA)
	if !hasSameAs {
		t.Error("expected A owl:sameAs B (same key values)")
	}
}

func TestOWLRL_HasKeyNoMatch(t *testing.T) {
	g := graph.NewGraph()
	keyProp := ex.Term("ssn")
	cls := ex.Term("Person")

	keys := buildRDFList(g, []term.Term{keyProp})
	g.Add(cls, namespace.OWL.HasKey, keys)

	g.Add(exA, namespace.RDF.Type, cls)
	g.Add(exB, namespace.RDF.Type, cls)
	g.Add(exA, keyProp, term.NewLiteral("111"))
	g.Add(exB, keyProp, term.NewLiteral("222"))

	OWLRLClosure(g)

	hasSameAs := g.Contains(exA, namespace.OWL.SameAs, exB) ||
		g.Contains(exB, namespace.OWL.SameAs, exA)
	if hasSameAs {
		t.Error("should NOT have sameAs (different key values)")
	}
}

// --- prp-irp: irreflexive property (consistency) ---

func TestOWLRL_IrreflexiveProperty(t *testing.T) {
	g := graph.NewGraph()
	g.Add(exP, namespace.RDF.Type, namespace.OWL.IrreflexiveProperty)
	g.Add(exA, exP, exA)

	_, incon := OWLRLClosureCheck(g)

	found := false
	for _, i := range incon {
		if i.Rule == "prp-irp" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected prp-irp inconsistency for reflexive use of irreflexive property")
	}
}

func TestOWLRL_IrreflexivePropertyNoViolation(t *testing.T) {
	g := graph.NewGraph()
	g.Add(exP, namespace.RDF.Type, namespace.OWL.IrreflexiveProperty)
	g.Add(exA, exP, exB)

	_, incon := OWLRLClosureCheck(g)

	for _, i := range incon {
		if i.Rule == "prp-irp" {
			t.Error("should not have prp-irp inconsistency")
		}
	}
}

// --- prp-asyp: asymmetric property (consistency) ---

func TestOWLRL_AsymmetricProperty(t *testing.T) {
	g := graph.NewGraph()
	g.Add(exP, namespace.RDF.Type, namespace.OWL.AsymmetricProperty)
	g.Add(exA, exP, exB)
	g.Add(exB, exP, exA)

	_, incon := OWLRLClosureCheck(g)

	found := false
	for _, i := range incon {
		if i.Rule == "prp-asyp" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected prp-asyp inconsistency")
	}
}

// --- prp-pdw: property disjoint with (consistency) ---

func TestOWLRL_PropertyDisjointWith(t *testing.T) {
	g := graph.NewGraph()
	g.Add(exP, namespace.OWL.PropertyDisjointWith, exQ)
	g.Add(exA, exP, exB)
	g.Add(exA, exQ, exB)

	_, incon := OWLRLClosureCheck(g)

	found := false
	for _, i := range incon {
		if i.Rule == "prp-pdw" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected prp-pdw inconsistency")
	}
}

// --- prp-adp: all disjoint properties (consistency) ---

func TestOWLRL_AllDisjointProperties(t *testing.T) {
	g := graph.NewGraph()
	adpNode := term.NewBNode("")
	g.Add(adpNode, namespace.RDF.Type, namespace.OWL.AllDisjointProperties)
	memberList := buildRDFList(g, []term.Term{exP, exQ, exR})
	g.Add(adpNode, namespace.OWL.Members, memberList)

	g.Add(exA, exP, exB)
	g.Add(exA, exQ, exB) // violates disjointness with P

	_, incon := OWLRLClosureCheck(g)

	found := false
	for _, i := range incon {
		if i.Rule == "prp-adp" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected prp-adp inconsistency")
	}
}

// --- Helper: build an RDF list in the graph ---

func buildRDFList(g *graph.Graph, items []term.Term) term.Subject {
	if len(items) == 0 {
		return namespace.RDF.Nil
	}
	var head term.Subject
	var prev term.Subject
	for i, item := range items {
		node := term.NewBNode("")
		if i == 0 {
			head = node
		}
		g.Add(node, namespace.RDF.First, item)
		if prev != nil {
			g.Add(prev, namespace.RDF.Rest, node)
		}
		prev = node
	}
	g.Add(prev, namespace.RDF.Rest, namespace.RDF.Nil)
	return head
}
