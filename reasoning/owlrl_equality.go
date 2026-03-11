package reasoning

import (
	"github.com/tggo/goRDFlib/namespace"
	"github.com/tggo/goRDFlib/term"
)

// applyEqualityRules implements eq-sym, eq-trans, eq-rep-s/p/o.
func (e *owlrlEngine) applyEqualityRules(allTriples []ruleTriple, emit emitFunc) {
	sameAs := namespace.OWL.SameAs

	// eq-sym: ?x owl:sameAs ?y → ?y owl:sameAs ?x
	// eq-trans: handled via union-find, emit all pairs
	classes := e.uf.classes()
	for _, members := range classes {
		for i := 0; i < len(members); i++ {
			for j := i + 1; j < len(members); j++ {
				si, oki := members[i].(term.Subject)
				sj, okj := members[j].(term.Subject)
				if oki && okj {
					emit(si, sameAs, members[j])
					emit(sj, sameAs, members[i])
				}
			}
		}
	}

	// eq-rep-s/p/o: substitute sameAs equivalents
	if e.uf.size() > 0 {
		memberOf := make(map[string][]term.Term, len(classes)*2)
		for _, members := range classes {
			for _, m := range members {
				memberOf[term.TermKey(m)] = members
			}
		}

		for _, t := range allTriples {
			sk := term.TermKey(t.s)
			objKey := term.TermKey(t.o)
			sEquivs := memberOf[sk]
			oEquivs := memberOf[objKey]

			// eq-rep-s
			for _, se := range sEquivs {
				if term.TermKey(se) == sk {
					continue
				}
				if subj, ok := se.(term.Subject); ok {
					emit(subj, t.p, t.o)
				}
			}
			// eq-rep-o
			for _, oe := range oEquivs {
				if term.TermKey(oe) == objKey {
					continue
				}
				emit(t.s, t.p, oe)
			}
			// Both substituted
			for _, se := range sEquivs {
				if term.TermKey(se) == sk {
					continue
				}
				subj, isSubj := se.(term.Subject)
				if !isSubj {
					continue
				}
				for _, oe := range oEquivs {
					if term.TermKey(oe) == objKey {
						continue
					}
					emit(subj, t.p, oe)
				}
			}
		}
	}
}
