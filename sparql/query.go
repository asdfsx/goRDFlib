package sparql

import (
	"fmt"

	rdflibgo "github.com/tggo/goRDFlib"
)

// Query executes a SPARQL query against the graph.
// Ported from: rdflib.graph.Graph.query → SPARQLProcessor.query
func Query(g *rdflibgo.Graph, query string, initBindings ...map[string]rdflibgo.Term) (*Result, error) {
	q, err := Parse(query)
	if err != nil {
		return nil, fmt.Errorf("sparql parse error: %w", err)
	}

	var bindings map[string]rdflibgo.Term
	if len(initBindings) > 0 {
		bindings = initBindings[0]
	}

	return EvalQuery(g, q, bindings)
}

// Update executes a SPARQL Update request against a dataset.
func Update(ds *Dataset, update string) error {
	u, err := ParseUpdate(update)
	if err != nil {
		return fmt.Errorf("sparql update parse error: %w", err)
	}
	return EvalUpdate(ds, u)
}
