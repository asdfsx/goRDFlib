package rdflibgo

import (
	"bytes"
	"strings"
	"testing"
)

// --- SPARQL parser: readStringLiteral with datatype/lang ---

func TestSPARQLQueryWithLangLiteral(t *testing.T) {
	g := NewGraph()
	g.Bind("ex", NewURIRefUnsafe("http://example.org/"))
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/name")
	g.Add(s, p, NewLiteral("hello", WithLang("en")))

	r, err := g.Query(`PREFIX ex: <http://example.org/> SELECT ?n WHERE { ?s ex:name "hello"@en }`)
	_ = r
	_ = err
}

func TestSPARQLQueryWithDatatypeLiteral(t *testing.T) {
	g := NewGraph()
	g.Bind("ex", NewURIRefUnsafe("http://example.org/"))
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/val")
	g.Add(s, p, NewLiteral("42", WithDatatype(XSDInteger)))

	r, err := g.Query(`PREFIX ex: <http://example.org/> SELECT ?s WHERE { ?s ex:val "42"^^<http://www.w3.org/2001/XMLSchema#integer> }`)
	_ = r
	_ = err
}

func TestSPARQLQueryTripleQuotedLiteral(t *testing.T) {
	g := NewGraph()
	g.Bind("ex", NewURIRefUnsafe("http://example.org/"))
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/desc")
	g.Add(s, p, NewLiteral("multi\nline"))

	r, err := g.Query(`PREFIX ex: <http://example.org/> SELECT ?s WHERE { ?s ex:desc """multi
line""" }`)
	_ = r
	_ = err
}

func TestSPARQLQuerySingleQuotedLiteral(t *testing.T) {
	g := NewGraph()
	g.Bind("ex", NewURIRefUnsafe("http://example.org/"))
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/name")
	g.Add(s, p, NewLiteral("hello"))

	r, err := g.Query(`PREFIX ex: <http://example.org/> SELECT ?s WHERE { ?s ex:name 'hello' }`)
	_ = r
	_ = err
}

// --- SPARQL parser: readTermOrVar edge cases ---

func TestSPARQLQueryDecimalLiteral(t *testing.T) {
	g := NewGraph()
	g.Bind("ex", NewURIRefUnsafe("http://example.org/"))
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/val")
	g.Add(s, p, NewLiteral("3.14", WithDatatype(XSDDecimal)))

	r, err := g.Query(`PREFIX ex: <http://example.org/> SELECT ?s WHERE { ?s ex:val 3.14 }`)
	_ = r
	_ = err
}

func TestSPARQLQueryDoubleLiteral(t *testing.T) {
	g := NewGraph()
	g.Bind("ex", NewURIRefUnsafe("http://example.org/"))
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/val")
	g.Add(s, p, NewLiteral("1.5e2", WithDatatype(XSDDouble)))

	r, err := g.Query(`PREFIX ex: <http://example.org/> SELECT ?s WHERE { ?s ex:val 1.5e2 }`)
	_ = r
	_ = err
}

// --- SPARQL parser: comment in query ---

func TestSPARQLQueryComment(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := g.Query(`
		# This is a comment
		PREFIX ex: <http://example.org/>
		SELECT ?name WHERE {
			?s ex:name ?name . # inline comment
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 3 {
		t.Errorf("expected 3, got %d", len(r.Bindings))
	}
}

// --- SPARQL: $var syntax ---

func TestSPARQLDollarVarSyntax(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := g.Query(`
		PREFIX ex: <http://example.org/>
		SELECT $name WHERE { $s ex:name $name }
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 3 {
		t.Errorf("expected 3, got %d", len(r.Bindings))
	}
}

// --- Turtle serializer: label for Variable (edge case) ---

func TestTurtleSerializerClassSubject(t *testing.T) {
	g := NewGraph()
	g.Bind("ex", NewURIRefUnsafe("http://example.org/"))
	g.Bind("rdfs", NewURIRefUnsafe(RDFSNamespace))
	cls, _ := NewURIRef("http://example.org/MyClass")
	g.Add(cls, RDF.Type, RDFS.Class)

	var buf bytes.Buffer
	g.Serialize(&buf, WithSerializeFormat("turtle"))
	if !strings.Contains(buf.String(), " a ") {
		t.Errorf("expected 'a' shorthand for rdf:type")
	}
}

// --- SPARQL EBV: integer, decimal, langString ---

func TestEffectiveBooleanValueInteger(t *testing.T) {
	if effectiveBooleanValue(NewLiteral(0)) {
		t.Error("0 should be false")
	}
	if !effectiveBooleanValue(NewLiteral(1)) {
		t.Error("1 should be true")
	}
}

func TestEffectiveBooleanValueDecimal(t *testing.T) {
	if effectiveBooleanValue(NewLiteral("0.0", WithDatatype(XSDDecimal))) {
		t.Error("0.0 should be false")
	}
	if !effectiveBooleanValue(NewLiteral("1.5", WithDatatype(XSDDecimal))) {
		t.Error("1.5 should be true")
	}
}

func TestEffectiveBooleanValueLangString(t *testing.T) {
	if !effectiveBooleanValue(NewLiteral("hello", WithLang("en"))) {
		t.Error("non-empty lang string should be true")
	}
}

func TestEffectiveBooleanValueURIRef(t *testing.T) {
	u, _ := NewURIRef("http://example.org/")
	if !effectiveBooleanValue(u) {
		t.Error("URIRef should be true")
	}
}

func TestEffectiveBooleanValueNil(t *testing.T) {
	if effectiveBooleanValue(nil) {
		t.Error("nil should be false")
	}
}

// --- toFloat64 / isIntegral / termString nil paths ---

func TestToFloat64Nil(t *testing.T) {
	if toFloat64(nil) != 0 {
		t.Error("nil should be 0")
	}
}

func TestToFloat64URIRef(t *testing.T) {
	u, _ := NewURIRef("http://example.org/")
	if toFloat64(u) != 0 {
		t.Error("URIRef should be 0")
	}
}

func TestIsIntegralNonLiteral(t *testing.T) {
	u, _ := NewURIRef("http://example.org/")
	if isIntegral(u) {
		t.Error("URIRef should not be integral")
	}
}

func TestTermStringNil(t *testing.T) {
	if termString(nil) != "" {
		t.Error("nil should be empty")
	}
}

// --- Turtle parser: resolveIRI with absolute ---

func TestTurtleParserAbsoluteIRINoResolve(t *testing.T) {
	g := parseTurtle(t, `
		@base <http://example.org/> .
		<http://other.org/s> <http://other.org/p> "v" .
	`)
	if g.Len() != 1 {
		t.Errorf("expected 1, got %d", g.Len())
	}
}

// --- SPARQL: resolveTermRef nil case ---

func TestResolveTermRefNil(t *testing.T) {
	prefixes := map[string]string{}
	got := resolveTermRef("unknown:foo", prefixes)
	if got != nil {
		t.Error("expected nil for unknown prefix")
	}
}

// --- SPARQL DISTINCT with explicit vars ---

func TestSPARQLDistinctExplicitVars(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := g.Query(`
		PREFIX ex: <http://example.org/>
		SELECT DISTINCT ?type WHERE { ?s a ?type }
		ORDER BY ?type
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Errorf("expected 1, got %d", len(r.Bindings))
	}
}

// --- matchKeywordCI: at EOF ---

func TestTurtleParserMatchKeywordAtEnd(t *testing.T) {
	// PREFIX at very end of input with no trailing whitespace
	g := NewGraph()
	err := g.Parse(strings.NewReader("PREFIX ex: <http://example.org/>\nex:s ex:p \"v\" ."), WithFormat("turtle"))
	if err != nil {
		t.Fatal(err)
	}
}

// --- Serializer: label for BNode ---

func TestTurtleSerializerBNodeLabel(t *testing.T) {
	g := NewGraph()
	g.Bind("ex", NewURIRefUnsafe("http://example.org/"))
	b := NewBNode("mybn")
	p1, _ := NewURIRef("http://example.org/p1")
	p2, _ := NewURIRef("http://example.org/p2")
	s, _ := NewURIRef("http://example.org/s")
	s2, _ := NewURIRef("http://example.org/s2")
	// BNode referenced > 1 times → must use _:mybn label
	g.Add(s, p1, b)
	g.Add(s2, p1, b)
	g.Add(b, p2, NewLiteral("v"))

	var buf bytes.Buffer
	g.Serialize(&buf, WithSerializeFormat("turtle"))
	out := buf.String()
	if !strings.Contains(out, "_:mybn") {
		t.Errorf("expected _:mybn for referenced BNode, got:\n%s", out)
	}
}
