package rdflibgo

import (
	"bytes"
	"strings"
	"testing"
)

// Ported from: test/test_w3c_spec/test_nquads_w3c.py, test/test_parsers/test_nquads.py

func TestNQSerializerBasic(t *testing.T) {
	// Ported from: rdflib.plugins.serializers.nquads.NQuadsSerializer
	g := NewGraph()
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	g.Add(s, p, NewLiteral("hello"))

	var buf bytes.Buffer
	if err := g.Serialize(&buf, WithSerializeFormat("nquads")); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "<http://example.org/s>") {
		t.Errorf("expected IRI, got:\n%s", out)
	}
}

func TestNQParserBasic(t *testing.T) {
	// Ported from: rdflib.plugins.parsers.nquads.NQuadsParser
	input := `<http://example.org/s> <http://example.org/p> "hello" <http://example.org/g> .
<http://example.org/s> <http://example.org/p2> "world" .
`
	g := NewGraph()
	if err := g.Parse(strings.NewReader(input), WithFormat("nquads")); err != nil {
		t.Fatal(err)
	}
	if g.Len() != 2 {
		t.Errorf("expected 2, got %d", g.Len())
	}
}

func TestNQParserComments(t *testing.T) {
	input := `# comment
<http://example.org/s> <http://example.org/p> "hello" .
`
	g := NewGraph()
	g.Parse(strings.NewReader(input), WithFormat("nquads"))
	if g.Len() != 1 {
		t.Errorf("expected 1, got %d", g.Len())
	}
}

func TestNQRoundtrip(t *testing.T) {
	g1 := NewGraph()
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	g1.Add(s, p, NewLiteral("hello"))

	var buf bytes.Buffer
	g1.Serialize(&buf, WithSerializeFormat("nquads"))

	g2 := NewGraph()
	g2.Parse(strings.NewReader(buf.String()), WithFormat("nquads"))

	if g1.Len() != g2.Len() {
		t.Errorf("roundtrip: %d vs %d", g1.Len(), g2.Len())
	}
}
