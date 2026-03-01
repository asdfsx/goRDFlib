package rdflibgo

import (
	"fmt"
	"io"
	"slices"
)

// NQuadsSerializer serializes quads to N-Quads format.
// Ported from: rdflib.plugins.serializers.nquads.NQuadsSerializer
type NQuadsSerializer struct{}

func init() {
	RegisterSerializer("nquads", func() Serializer { return &NQuadsSerializer{} })
	RegisterSerializer("nq", func() Serializer { return &NQuadsSerializer{} })
}

func (s *NQuadsSerializer) Serialize(g *Graph, w io.Writer, base string) error {
	var lines []string
	g.Triples(nil, nil, nil)(func(t Triple) bool {
		line := ntTerm(t.Subject) + " " + ntTerm(t.Predicate) + " " + ntTerm(t.Object)
		// Add graph context if the graph has an identifier that's a URIRef
		if id, ok := g.Identifier().(URIRef); ok {
			line += " " + ntTerm(id)
		}
		line += " ."
		lines = append(lines, line)
		return true
	})
	slices.Sort(lines)
	for _, line := range lines {
		if _, err := fmt.Fprintln(w, line); err != nil {
			return err
		}
	}
	return nil
}
