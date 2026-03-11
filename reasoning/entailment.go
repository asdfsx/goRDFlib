package reasoning

import (
	"errors"

	"github.com/tggo/goRDFlib/graph"
)

// Regime identifies an entailment regime.
type Regime int

const (
	// RDFS is the RDFS entailment regime applying rules rdfs2, rdfs3, rdfs5, rdfs7, rdfs9, rdfs11.
	RDFS Regime = iota + 1
)

// ErrUnknownRegime is returned when an unrecognized entailment regime is requested.
var ErrUnknownRegime = errors.New("reasoning: unknown entailment regime")

// Expand applies the given entailment regime to the graph, adding inferred triples.
// Returns the number of triples added and any error.
func Expand(g *graph.Graph, r Regime) (int, error) {
	switch r {
	case RDFS:
		return RDFSClosure(g), nil
	default:
		return 0, ErrUnknownRegime
	}
}
