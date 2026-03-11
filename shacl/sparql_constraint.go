package shacl

import "github.com/tggo/goRDFlib/graph"

// SPARQLConstraint implements sh:sparql on shapes. Each focus node is validated
// by running a SELECT query with $this pre-bound. Each result row is a violation.
type SPARQLConstraint struct {
	Node        Term   // the blank/IRI node of the sh:sparql object
	Select      string // the sh:select query body
	Prefixes    string // resolved PREFIX preamble
	Messages    []Term // sh:message on the constraint
	Deactivated bool   // sh:deactivated
}

func (c *SPARQLConstraint) ComponentIRI() string {
	return SH + "SPARQLConstraintComponent"
}

func (c *SPARQLConstraint) Evaluate(ctx *evalContext, shape *Shape, focusNode Term, valueNodes []Term) []ValidationResult {
	if c.Deactivated {
		return nil
	}

	// Build the full query with prefixes, normalizing $var to ?var
	fullQuery := normalizeDollarVars(c.Prefixes + c.Select)

	// Pre-bind variables via textual substitution
	bindings := map[string]string{
		"this": termToSPARQL(focusNode),
	}
	if shape.Path != nil && shape.Path.Kind == PathPredicate {
		bindings["PATH"] = termToSPARQL(shape.Path.Pred)
	}
	bindings["currentShape"] = termToSPARQL(shape.ID)
	if ctx.shapesGraph != nil && ctx.shapesGraph.baseURI != "" {
		bindings["shapesGraph"] = "<" + ctx.shapesGraph.baseURI + ">"
	}

	query := preBindQuery(fullQuery, bindings)

	// Provide shapes graph as a named graph for GRAPH ?shapesGraph { ... }
	var namedGraphs map[string]*graph.Graph
	if ctx.shapesGraph != nil && ctx.shapesGraph.g != nil && ctx.shapesGraph.baseURI != "" {
		namedGraphs = map[string]*graph.Graph{
			ctx.shapesGraph.baseURI: ctx.shapesGraph.g,
		}
	}

	rows, err := executeSPARQL(ctx.dataGraph, query, nil, namedGraphs)
	if err != nil {
		r := makeResult(shape, focusNode, focusNode, c.ComponentIRI())
		r.SourceConstraint = c.Node
		if len(c.Messages) > 0 {
			r.ResultMessages = c.Messages
		}
		return []ValidationResult{r}
	}

	var results []ValidationResult
	for _, row := range rows {
		value := focusNode
		if v, ok := row["value"]; ok && !v.IsNone() {
			value = v
		}

		r := makeResult(shape, focusNode, value, c.ComponentIRI())
		r.SourceConstraint = c.Node

		if p, ok := row["path"]; ok && !p.IsNone() {
			r.ResultPath = p
		}

		if len(c.Messages) > 0 {
			r.ResultMessages = c.Messages
		}

		results = append(results, r)
	}
	return results
}
