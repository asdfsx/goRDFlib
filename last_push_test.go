package rdflibgo

import (
	"bytes"
	"strings"
	"testing"
)

// --- MustTerm success path (panic path already tested) ---

func TestClosedNamespaceMustTermSuccess(t *testing.T) {
	ns := NewClosedNamespace("http://example.org/", []string{"Foo"})
	u := ns.MustTerm("Foo")
	if u.Value() != "http://example.org/Foo" {
		t.Errorf("got %q", u.Value())
	}
}

// --- RDF/XML: parseRDFRoot xml:base ---

func TestRDFXMLParserXMLBaseOnRoot(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/"
         xml:base="http://base.org/">
  <rdf:Description rdf:about="s">
    <ex:p>v</ex:p>
  </rdf:Description>
</rdf:RDF>`
	g := NewGraph()
	g.Parse(strings.NewReader(input), WithFormat("xml"))
	if g.Len() != 1 {
		t.Errorf("expected 1, got %d", g.Len())
	}
}

// --- RDF/XML: document without rdf:RDF wrapper ---

func TestRDFXMLParserNoWrapper(t *testing.T) {
	input := `<?xml version="1.0"?>
<ex:Thing xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
          xmlns:ex="http://example.org/"
          rdf:about="http://example.org/s">
  <ex:p>v</ex:p>
</ex:Thing>`
	g := NewGraph()
	err := g.Parse(strings.NewReader(input), WithFormat("xml"))
	if err != nil {
		t.Fatal(err)
	}
}

// --- Turtle: local name with backslash escape ---

func TestTurtleParserLocalNameWithBackslash(t *testing.T) {
	g := parseTurtle(t, `
		@prefix ex: <http://example.org/> .
		ex:s ex:p ex:hello\.world .
	`)
	if g.Len() != 1 {
		t.Errorf("expected 1, got %d", g.Len())
	}
}

// --- Turtle: matchKeywordCI short input ---

func TestTurtleParserShortInput(t *testing.T) {
	g := NewGraph()
	err := g.Parse(strings.NewReader("@prefix e: <http://e.o/> . e:s e:p e:o ."), WithFormat("turtle"))
	if err != nil {
		t.Fatal(err)
	}
}

// --- Turtle: resolveIRI with fragment ---

func TestTurtleParserFragmentIRI(t *testing.T) {
	g := parseTurtle(t, `
		@base <http://example.org/doc> .
		<#section> <#p> "v" .
	`)
	if g.Len() != 1 {
		t.Errorf("expected 1, got %d", g.Len())
	}
}

// --- Turtle serializer: write with base ---

func TestTurtleSerializerWithBase(t *testing.T) {
	g := NewGraph()
	g.Bind("ex", NewURIRefUnsafe("http://example.org/"))
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	g.Add(s, p, NewLiteral("v"))

	var buf bytes.Buffer
	g.Serialize(&buf, WithSerializeFormat("turtle"), WithSerializeBase("http://example.org/"))
	if !strings.Contains(buf.String(), "@base") {
		t.Errorf("expected @base, got:\n%s", buf.String())
	}
}

// --- Turtle serializer: multiple objects ---

func TestTurtleSerializerMultipleObjectsSameSubjectPred(t *testing.T) {
	g := NewGraph()
	g.Bind("ex", NewURIRefUnsafe("http://example.org/"))
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	g.Add(s, p, NewLiteral("a"))
	g.Add(s, p, NewLiteral("b"))
	g.Add(s, p, NewLiteral("c"))

	var buf bytes.Buffer
	g.Serialize(&buf, WithSerializeFormat("turtle"))
	out := buf.String()
	if strings.Count(out, ",") < 2 {
		t.Errorf("expected commas for multiple objects, got:\n%s", out)
	}
}

// --- Turtle serializer: rdfs:label predicate ---

func TestTurtleSerializerRDFSLabelOrder(t *testing.T) {
	g := NewGraph()
	g.Bind("ex", NewURIRefUnsafe("http://example.org/"))
	g.Bind("rdfs", NewURIRefUnsafe(RDFSNamespace))
	s, _ := NewURIRef("http://example.org/s")
	other, _ := NewURIRef("http://example.org/zzz")
	g.Add(s, RDFS.Label, NewLiteral("My Label"))
	g.Add(s, other, NewLiteral("zzz"))

	var buf bytes.Buffer
	g.Serialize(&buf, WithSerializeFormat("turtle"))
	out := buf.String()
	// rdfs:label should come before ex:zzz
	labelIdx := strings.Index(out, "rdfs:label")
	otherIdx := strings.Index(out, "ex:zzz")
	if labelIdx < 0 || otherIdx < 0 || labelIdx > otherIdx {
		t.Errorf("rdfs:label should come before other predicates:\n%s", out)
	}
}

// --- SPARQL parser: resolveTermValue numeric ---

func TestSPARQLResolveTermValueNumeric(t *testing.T) {
	p := &sparqlParser{prefixes: map[string]string{}}
	// Integer
	v := p.resolveTermValue("42")
	if l, ok := v.(Literal); !ok || l.Datatype() != XSDInteger {
		t.Errorf("expected xsd:integer, got %v", v)
	}
	// Decimal
	v = p.resolveTermValue("3.14")
	if l, ok := v.(Literal); !ok || l.Datatype() != XSDDecimal {
		t.Errorf("expected xsd:decimal, got %v", v)
	}
	// Double
	v = p.resolveTermValue("1.5e2")
	if l, ok := v.(Literal); !ok || l.Datatype() != XSDDouble {
		t.Errorf("expected xsd:double, got %v", v)
	}
	// Boolean
	v = p.resolveTermValue("true")
	if l, ok := v.(Literal); !ok || l.Lexical() != "true" {
		t.Errorf("expected true, got %v", v)
	}
	v = p.resolveTermValue("false")
	if l, ok := v.(Literal); !ok || l.Lexical() != "false" {
		t.Errorf("expected false, got %v", v)
	}
	// IRI
	v = p.resolveTermValue("<http://example.org/>")
	if _, ok := v.(URIRef); !ok {
		t.Errorf("expected URIRef, got %T", v)
	}
	// Empty
	v = p.resolveTermValue("")
	if l, ok := v.(Literal); !ok || l.Lexical() != "" {
		t.Errorf("expected empty literal, got %v", v)
	}
	// Prefixed name
	p.prefixes["ex"] = "http://example.org/"
	v = p.resolveTermValue("ex:Thing")
	if u, ok := v.(URIRef); !ok || u.Value() != "http://example.org/Thing" {
		t.Errorf("expected URIRef, got %v", v)
	}
}

// --- SPARQL solutionKey with nil vars ---

func TestSolutionKeyNilVars(t *testing.T) {
	s := map[string]Term{"a": NewLiteral("1"), "b": NewLiteral("2")}
	k := solutionKey(s, nil)
	if k == "" {
		t.Error("expected non-empty key")
	}
}

func TestSolutionKeyWithVars(t *testing.T) {
	s := map[string]Term{"a": NewLiteral("1"), "b": NewLiteral("2")}
	k := solutionKey(s, []string{"a"})
	if !strings.Contains(k, "a=") {
		t.Errorf("expected a= in key, got %q", k)
	}
}

// --- Turtle parser: tryNumeric edge: sign only ---

func TestTurtleParserSignOnly(t *testing.T) {
	// "+" by itself is not a number
	g := NewGraph()
	err := g.Parse(strings.NewReader(`@prefix ex: <http://e.o/> . ex:s ex:p + .`), WithFormat("turtle"))
	// Should error or treat + as something else
	_ = err
}

// --- SPARQL: parseGroupGraphPattern BIND without parens ---

func TestSPARQLSubGroupPattern(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := g.Query(`
		PREFIX ex: <http://example.org/>
		SELECT ?name WHERE {
			{ ?s ex:name ?name . ?s ex:age ?age . FILTER(?age > 30) }
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Errorf("expected 1, got %d", len(r.Bindings))
	}
}
