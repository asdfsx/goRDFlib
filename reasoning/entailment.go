package reasoning

import (
	"errors"

	"github.com/tggo/goRDFlib/graph"
)

// Regime identifies an entailment regime. Values are bitmask flags
// so they can be combined: Expand(g, RDFS|OWLRL).
type Regime int

const (
	// RDFS is the RDFS entailment regime applying rules rdfs2, rdfs3, rdfs5, rdfs7, rdfs9, rdfs11.
	RDFS Regime = 1 << iota
	// OWLRL is the OWL 2 RL entailment regime applying property, equality, and class axiom rules.
	// When used alone, RDFS closure is applied first automatically.
	OWLRL
)

// ErrUnknownRegime is returned when an unrecognized entailment regime is requested.
var ErrUnknownRegime = errors.New("reasoning: unknown entailment regime")

// Expand applies the given entailment regime(s) to the graph, adding inferred triples.
// Regimes can be combined with |: Expand(g, RDFS|OWLRL).
// Returns the number of triples added and any error.
func Expand(g *graph.Graph, r Regime) (int, error) {
	n, _, err := ExpandCheck(g, r)
	return n, err
}

// ExpandCheck applies entailment regime(s) and checks for OWL 2 RL inconsistencies.
// Returns the number of triples added, detected inconsistencies, and any error.
func ExpandCheck(g *graph.Graph, r Regime) (int, []Inconsistency, error) {
	known := RDFS | OWLRL
	if r == 0 || r&^known != 0 {
		return 0, nil, ErrUnknownRegime
	}
	total := 0
	// RDFS is always applied first (OWL RL builds on RDFS).
	if r&RDFS != 0 || r&OWLRL != 0 {
		total += RDFSClosure(g)
	}
	var incon []Inconsistency
	if r&OWLRL != 0 {
		n, ic := OWLRLClosureCheck(g)
		total += n
		incon = ic
	}
	return total, incon, nil
}
