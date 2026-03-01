package rdflibgo

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

// Ported from: test/test_w3c_spec/test_nt_w3c.py, test/test_nt_misc.py

func TestNTSerializerBasic(t *testing.T) {
	// Ported from: rdflib.plugins.serializers.nt.NTSerializer
	g := NewGraph()
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	g.Add(s, p, NewLiteral("hello"))

	var buf bytes.Buffer
	if err := g.Serialize(&buf, WithSerializeFormat("nt")); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "<http://example.org/s>") {
		t.Errorf("expected full IRI, got:\n%s", out)
	}
	if !strings.Contains(out, `"hello"`) {
		t.Errorf("expected literal, got:\n%s", out)
	}
	if !strings.HasSuffix(strings.TrimSpace(out), ".") {
		t.Errorf("expected trailing dot, got:\n%s", out)
	}
}

func TestNTSerializerEscaping(t *testing.T) {
	// Ported from: rdflib N-Triples escape handling
	g := NewGraph()
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	g.Add(s, p, NewLiteral("line1\nline2"))

	var buf bytes.Buffer
	g.Serialize(&buf, WithSerializeFormat("nt"))
	if !strings.Contains(buf.String(), `\n`) {
		t.Errorf("expected escaped newline, got:\n%s", buf.String())
	}
}

func TestNTSerializerLangAndDatatype(t *testing.T) {
	g := NewGraph()
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	g.Add(s, p, NewLiteral("hello", WithLang("en")))
	g.Add(s, p, NewLiteral("42", WithDatatype(XSDInteger)))

	var buf bytes.Buffer
	g.Serialize(&buf, WithSerializeFormat("nt"))
	out := buf.String()
	if !strings.Contains(out, `"hello"@en`) {
		t.Errorf("expected lang tag, got:\n%s", out)
	}
	if !strings.Contains(out, `"42"^^<http://www.w3.org/2001/XMLSchema#integer>`) {
		t.Errorf("expected datatype, got:\n%s", out)
	}
}

func TestNTSerializerDeterministic(t *testing.T) {
	g := NewGraph()
	s, _ := NewURIRef("http://example.org/s")
	for i := 0; i < 5; i++ {
		p, _ := NewURIRef(fmt.Sprintf("http://example.org/p%d", i))
		g.Add(s, p, NewLiteral(fmt.Sprintf("v%d", i)))
	}
	var buf1, buf2 bytes.Buffer
	g.Serialize(&buf1, WithSerializeFormat("nt"))
	g.Serialize(&buf2, WithSerializeFormat("nt"))
	if buf1.String() != buf2.String() {
		t.Error("N-Triples output not deterministic")
	}
}

func TestNTParserBasic(t *testing.T) {
	// Ported from: rdflib.plugins.parsers.ntriples.NTriplesParser
	input := `<http://example.org/s> <http://example.org/p> "hello" .
<http://example.org/s> <http://example.org/p2> <http://example.org/o> .
`
	g := NewGraph()
	if err := g.Parse(strings.NewReader(input), WithFormat("nt")); err != nil {
		t.Fatal(err)
	}
	if g.Len() != 2 {
		t.Errorf("expected 2, got %d", g.Len())
	}
}

func TestNTParserBNode(t *testing.T) {
	input := `_:b1 <http://example.org/p> "hello" .
`
	g := NewGraph()
	if err := g.Parse(strings.NewReader(input), WithFormat("nt")); err != nil {
		t.Fatal(err)
	}
	if g.Len() != 1 {
		t.Errorf("expected 1, got %d", g.Len())
	}
}

func TestNTParserLangTag(t *testing.T) {
	input := `<http://example.org/s> <http://example.org/p> "hello"@en .
`
	g := NewGraph()
	g.Parse(strings.NewReader(input), WithFormat("nt"))
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	val, ok := g.Value(s, &p, nil)
	if !ok {
		t.Fatal("expected value")
	}
	if lit, ok := val.(Literal); !ok || lit.Language() != "en" {
		t.Errorf("expected lang en, got %v", val)
	}
}

func TestNTParserDatatype(t *testing.T) {
	input := `<http://example.org/s> <http://example.org/p> "42"^^<http://www.w3.org/2001/XMLSchema#integer> .
`
	g := NewGraph()
	g.Parse(strings.NewReader(input), WithFormat("nt"))
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	val, ok := g.Value(s, &p, nil)
	if !ok {
		t.Fatal("expected value")
	}
	if lit, ok := val.(Literal); !ok || lit.Datatype() != XSDInteger {
		t.Errorf("expected xsd:integer, got %v", val)
	}
}

func TestNTParserComments(t *testing.T) {
	input := `# comment
<http://example.org/s> <http://example.org/p> "hello" .

# another comment
`
	g := NewGraph()
	g.Parse(strings.NewReader(input), WithFormat("nt"))
	if g.Len() != 1 {
		t.Errorf("expected 1, got %d", g.Len())
	}
}

func TestNTParserEscape(t *testing.T) {
	input := `<http://example.org/s> <http://example.org/p> "line1\nline2" .
`
	g := NewGraph()
	g.Parse(strings.NewReader(input), WithFormat("nt"))
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	val, _ := g.Value(s, &p, nil)
	if val.String() != "line1\nline2" {
		t.Errorf("expected newline in value, got %q", val.String())
	}
}

func TestNTRoundtrip(t *testing.T) {
	// Ported from: test/test_roundtrip.py — N-Triples roundtrip
	g1 := NewGraph()
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	g1.Add(s, p, NewLiteral("hello"))
	g1.Add(s, p, NewLiteral("world", WithLang("en")))
	g1.Add(s, p, NewLiteral("42", WithDatatype(XSDInteger)))

	var buf bytes.Buffer
	g1.Serialize(&buf, WithSerializeFormat("nt"))

	g2 := NewGraph()
	g2.Parse(strings.NewReader(buf.String()), WithFormat("nt"))

	if g1.Len() != g2.Len() {
		t.Errorf("roundtrip: %d vs %d", g1.Len(), g2.Len())
	}
}
