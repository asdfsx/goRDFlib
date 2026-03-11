package reasoning

import (
	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/namespace"
	"github.com/tggo/goRDFlib/term"
)

// RDFSClosure applies RDFS closure rules to the graph, adding inferred triples.
// Implements rules rdfs2, rdfs3, rdfs5, rdfs7, rdfs9, rdfs11.
// Returns the total number of triples added.
//
// Not safe for concurrent use.
func RDFSClosure(g *graph.Graph) int {
	e := newRDFSEngine(g)
	return e.run()
}

// rdfsEngine holds schema indexes and dedup state for RDFS closure computation.
type rdfsEngine struct {
	g *graph.Graph

	// Schema indexes
	domains    map[string][]term.URIRef  // predicate key → domain classes
	ranges     map[string][]term.URIRef  // predicate key → range classes
	subClassOf map[string][]term.URIRef  // class key → transitive superclasses
	subPropOf  map[string][]term.URIRef  // property key → transitive superproperties

	// Dedup
	existing map[string]struct{}
}

func newRDFSEngine(g *graph.Graph) *rdfsEngine {
	return &rdfsEngine{
		g:        g,
		existing: make(map[string]struct{}, g.Len()),
	}
}

func (e *rdfsEngine) run() int {
	// Build existing set from all triples
	e.g.Triples(nil, nil, nil)(func(t term.Triple) bool {
		e.existing[tripleKey(t.Subject, t.Predicate, t.Object)] = struct{}{}
		return true
	})

	// Build schema indexes
	e.buildSchemaIndexes()

	// Compute transitive closures
	e.closeTransitive()

	totalAdded := 0

	// Fixed-point loop
	for {
		newTriples := e.applyRules()
		if len(newTriples) == 0 {
			break
		}

		hasSchemaTriples := false
		for _, t := range newTriples {
			e.g.Add(t.Subject, t.Predicate, t.Object)
			pk := term.TermKey(t.Predicate)
			if pk == term.TermKey(namespace.RDFS.SubClassOf) ||
				pk == term.TermKey(namespace.RDFS.SubPropertyOf) ||
				pk == term.TermKey(namespace.RDFS.Domain) ||
				pk == term.TermKey(namespace.RDFS.Range) {
				hasSchemaTriples = true
			}
		}
		totalAdded += len(newTriples)

		if hasSchemaTriples {
			e.buildSchemaIndexes()
			e.closeTransitive()
		}
	}

	return totalAdded
}

func (e *rdfsEngine) buildSchemaIndexes() {
	e.domains = make(map[string][]term.URIRef)
	e.ranges = make(map[string][]term.URIRef)
	e.subClassOf = make(map[string][]term.URIRef)
	e.subPropOf = make(map[string][]term.URIRef)

	domainPred := namespace.RDFS.Domain
	rangePred := namespace.RDFS.Range
	subClassPred := namespace.RDFS.SubClassOf
	subPropPred := namespace.RDFS.SubPropertyOf

	e.g.Triples(nil, &domainPred, nil)(func(t term.Triple) bool {
		if c, ok := t.Object.(term.URIRef); ok {
			pk := term.TermKey(t.Subject)
			e.domains[pk] = append(e.domains[pk], c)
		}
		return true
	})

	e.g.Triples(nil, &rangePred, nil)(func(t term.Triple) bool {
		if c, ok := t.Object.(term.URIRef); ok {
			pk := term.TermKey(t.Subject)
			e.ranges[pk] = append(e.ranges[pk], c)
		}
		return true
	})

	e.g.Triples(nil, &subClassPred, nil)(func(t term.Triple) bool {
		if c, ok := t.Object.(term.URIRef); ok {
			sk := term.TermKey(t.Subject)
			e.subClassOf[sk] = append(e.subClassOf[sk], c)
		}
		return true
	})

	e.g.Triples(nil, &subPropPred, nil)(func(t term.Triple) bool {
		if c, ok := t.Object.(term.URIRef); ok {
			sk := term.TermKey(t.Subject)
			e.subPropOf[sk] = append(e.subPropOf[sk], c)
		}
		return true
	})
}

// closeTransitive computes transitive closures for subClassOf and subPropertyOf.
func (e *rdfsEngine) closeTransitive() {
	e.subClassOf = transitiveClose(e.subClassOf)
	e.subPropOf = transitiveClose(e.subPropOf)
}

// transitiveClose computes the transitive closure of a relation map.
func transitiveClose(m map[string][]term.URIRef) map[string][]term.URIRef {
	result := make(map[string][]term.URIRef, len(m))
	for k := range m {
		visited := make(map[string]struct{})
		var supers []term.URIRef
		var queue []term.URIRef
		queue = append(queue, m[k]...)
		for len(queue) > 0 {
			cur := queue[0]
			queue = queue[1:]
			ck := term.TermKey(cur)
			if _, seen := visited[ck]; seen {
				continue
			}
			visited[ck] = struct{}{}
			supers = append(supers, cur)
			queue = append(queue, m[ck]...)
		}
		if len(supers) > 0 {
			result[k] = supers
		}
	}
	return result
}

// applyRules applies rdfs2, rdfs3, rdfs7, rdfs9 and returns new triples to add.
func (e *rdfsEngine) applyRules() []term.Triple {
	var newTriples []term.Triple
	rdfType := namespace.RDF.Type

	e.g.Triples(nil, nil, nil)(func(t term.Triple) bool {
		pk := term.TermKey(t.Predicate)

		// rdfs2: ?p rdfs:domain ?C, ?s ?p ?o → ?s rdf:type ?C
		if domains, ok := e.domains[pk]; ok {
			for _, c := range domains {
				if e.addNew(t.Subject, rdfType, c) {
					newTriples = append(newTriples, term.Triple{Subject: t.Subject, Predicate: rdfType, Object: c})
				}
			}
		}

		// rdfs3: ?p rdfs:range ?C, ?s ?p ?o → ?o rdf:type ?C (only if o is a Subject)
		if ranges, ok := e.ranges[pk]; ok {
			if oSubj, ok := t.Object.(term.Subject); ok {
				for _, c := range ranges {
					if e.addNew(oSubj, rdfType, c) {
						newTriples = append(newTriples, term.Triple{Subject: oSubj, Predicate: rdfType, Object: c})
					}
				}
			}
		}

		// rdfs7: ?p rdfs:subPropertyOf ?q, ?s ?p ?o → ?s ?q ?o
		if superProps, ok := e.subPropOf[pk]; ok {
			for _, q := range superProps {
				if e.addNew(t.Subject, q, t.Object) {
					newTriples = append(newTriples, term.Triple{Subject: t.Subject, Predicate: q, Object: t.Object})
				}
			}
		}

		// rdfs9: ?s rdf:type ?C1, ?C1 rdfs:subClassOf ?C2 → ?s rdf:type ?C2
		if pk == term.TermKey(rdfType) {
			if c1, ok := t.Object.(term.URIRef); ok {
				c1k := term.TermKey(c1)
				if superClasses, ok := e.subClassOf[c1k]; ok {
					for _, c2 := range superClasses {
						if e.addNew(t.Subject, rdfType, c2) {
							newTriples = append(newTriples, term.Triple{Subject: t.Subject, Predicate: rdfType, Object: c2})
						}
					}
				}
			}
		}

		return true
	})

	return newTriples
}

// addNew checks if a triple is new and marks it as existing. Returns true if new.
func (e *rdfsEngine) addNew(s term.Subject, p term.URIRef, o term.Term) bool {
	k := tripleKey(s, p, o)
	if _, exists := e.existing[k]; exists {
		return false
	}
	e.existing[k] = struct{}{}
	return true
}

func tripleKey(s term.Subject, p term.URIRef, o term.Term) string {
	return term.TermKey(s) + "|" + term.TermKey(p) + "|" + term.TermKey(o)
}
