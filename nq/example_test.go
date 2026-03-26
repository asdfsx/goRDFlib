package nq_test

import (
	"fmt"
	"strings"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/nq"
	"github.com/tggo/goRDFlib/nt"
)

// NtToNq converts N-Triples to N-Quads by placing all triples
// into a named graph identified by graphURN.
//
// This is the rdflibgo equivalent of the knakk/rdf pattern:
//
//	dec := rdf.NewTripleDecoder(strings.NewReader(ntData), rdf.NTriples)
//	triples, _ := dec.DecodeAll()
//	for _, t := range triples { quad := rdf.Quad{Triple: t, Ctx: rdf.Context(iri)} }
func NtToNq(ntData, graphURN string) (string, error) {
	// Create a graph with the desired named graph identifier.
	// The key difference from the issue: use WithIdentifier so the
	// N-Quads serializer emits the graph name as the 4th component.
	graphIRI, err := rdflibgo.NewURIRef(graphURN)
	if err != nil {
		return "", fmt.Errorf("invalid graph URN %q: %w", graphURN, err)
	}
	g := rdflibgo.NewGraph(rdflibgo.WithIdentifier(graphIRI))

	// Parse N-Triples into the named graph.
	if err := nt.Parse(g, strings.NewReader(ntData)); err != nil {
		return "", fmt.Errorf("parsing N-Triples: %w", err)
	}

	// Serialize as N-Quads — the graph identifier is automatically
	// included as the 4th element of each quad.
	var buf strings.Builder
	if err := nq.Serialize(g, &buf); err != nil {
		return "", fmt.Errorf("serializing N-Quads: %w", err)
	}
	return buf.String(), nil
}

func Example_ntToNq() {
	ntData := `<http://example.org/s1> <http://example.org/p> "hello" .
<http://example.org/s2> <http://example.org/p> "world" .
`
	result, err := NtToNq(ntData, "urn:example:mygraph")
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Print(result)
	// Output:
	// <http://example.org/s1> <http://example.org/p> "hello" <urn:example:mygraph> .
	// <http://example.org/s2> <http://example.org/p> "world" <urn:example:mygraph> .
}

func Example_errorHandlerSkip() {
	// Data with an invalid IRI (space inside <>). The error handler
	// skips the bad line and parsing continues with the rest.
	ntData := `<http://example.org/s1> <http://example.org/p> "good" .
<http://example.org/s 2> <http://example.org/p> "bad iri" .
<http://example.org/s3> <http://example.org/p> "also good" .
`
	g := rdflibgo.NewGraph()
	err := nt.Parse(g, strings.NewReader(ntData), nt.WithErrorHandler(
		func(lineNum int, line string, err error) (string, bool) {
			fmt.Printf("skipping line %d: %v\n", lineNum, err)
			return "", false
		},
	))
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Printf("parsed %d triples\n", g.Len())
	// Output:
	// skipping line 2: line 2: subject: line 2: invalid character in IRI
	// parsed 2 triples
}

func Example_errorHandlerRetry() {
	// The error handler fixes the invalid IRI by percent-encoding
	// the space, then returns the fixed line for re-parsing.
	ntData := `<http://example.org/s> <http://example.org/p> <http://example.org/o with space> .
`
	g := rdflibgo.NewGraph()
	err := nt.Parse(g, strings.NewReader(ntData), nt.WithErrorHandler(
		func(lineNum int, line string, err error) (string, bool) {
			fixed := strings.ReplaceAll(line, "o with space", "o%20with%20space")
			return fixed, true
		},
	))
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Printf("parsed %d triples\n", g.Len())
	// Output:
	// parsed 1 triples
}

func Example_ntToNqDefaultGraph() {
	// Without WithIdentifier, triples go into the default graph
	// and N-Quads output has no 4th component (just triples with a dot).
	ntData := `<http://example.org/s> <http://example.org/p> "value" .
`
	g := rdflibgo.NewGraph()
	if err := nt.Parse(g, strings.NewReader(ntData)); err != nil {
		fmt.Println("error:", err)
		return
	}
	var buf strings.Builder
	if err := nq.Serialize(g, &buf); err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Print(buf.String())
	// Output:
	// <http://example.org/s> <http://example.org/p> "value" .
}
