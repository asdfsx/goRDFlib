package reasoning

import (
	"github.com/tggo/goRDFlib/namespace"
	"github.com/tggo/goRDFlib/term"
)

// applyClassRules implements cls-svf1/2, cls-avf, cls-hv1/2, cls-int1/2,
// cls-uni, cls-maxc1/2, cls-maxqc1-4, cls-oo.
func (e *owlrlEngine) applyClassRules(allTriples []ruleTriple, emit emitFunc) {
	rdfType := namespace.RDF.Type
	sameAs := namespace.OWL.SameAs

	// Build type index: subject key → set of type keys
	typeIndex := make(map[string]map[string]struct{})
	// Build property index: (subject key, property key) → objects
	type spKey struct{ subj, prop string }
	propObjs := make(map[spKey][]term.Term)
	// Build reverse property index: (property key, object key) → subjects
	type poKey struct{ prop, obj string }
	propSubjs := make(map[poKey][]term.Subject)

	for _, t := range allTriples {
		pk := term.TermKey(t.p)
		sk := term.TermKey(t.s)
		if pk == term.TermKey(rdfType) {
			if typeIndex[sk] == nil {
				typeIndex[sk] = make(map[string]struct{})
			}
			typeIndex[sk][term.TermKey(t.o)] = struct{}{}
		}
		propObjs[spKey{sk, pk}] = append(propObjs[spKey{sk, pk}], t.o)
		propSubjs[poKey{pk, term.TermKey(t.o)}] = append(propSubjs[poKey{pk, term.TermKey(t.o)}], t.s)
	}

	// --- cls-hv1: restriction hasValue y, onProperty p, u rdf:type restriction → u p y ---
	// --- cls-hv2: restriction hasValue y, onProperty p, u p y → u rdf:type restriction ---
	for _, r := range e.restrictions {
		if r.hasValue == nil {
			continue
		}
		rk := term.TermKey(r.node)
		pk := term.TermKey(r.onProperty)

		// cls-hv1: instances of the restriction get the property value
		for sk, types := range typeIndex {
			if _, ok := types[rk]; !ok {
				continue
			}
			// Find the subject term
			for _, t := range allTriples {
				if term.TermKey(t.s) == sk {
					emit(t.s, r.onProperty, r.hasValue)
					break
				}
			}
		}

		// cls-hv2: anything with (p, y) gets typed as the restriction
		hvk := term.TermKey(r.hasValue)
		for _, s := range propSubjs[poKey{pk, hvk}] {
			emit(s, rdfType, r.node)
		}
	}

	// --- cls-svf1: restriction svf C, onProperty p, u p v, v rdf:type C → u rdf:type restriction ---
	// --- cls-svf2: restriction svf owl:Thing, onProperty p, u p v → u rdf:type restriction ---
	for _, r := range e.restrictions {
		if r.someValuesFrom == nil {
			continue
		}
		rk := r.node
		pk := term.TermKey(r.onProperty)
		svfKey := term.TermKey(r.someValuesFrom)
		isThingKey := svfKey == term.TermKey(namespace.OWL.Thing)

		for _, t := range allTriples {
			if term.TermKey(t.p) != pk {
				continue
			}
			if isThingKey {
				// cls-svf2: any value suffices
				emit(t.s, rdfType, rk)
			} else {
				// cls-svf1: check if object has the required type
				objTypes := typeIndex[term.TermKey(t.o)]
				if _, ok := objTypes[svfKey]; ok {
					emit(t.s, rdfType, rk)
				}
			}
		}
	}

	// --- cls-avf: restriction avf C, onProperty p, u rdf:type restriction, u p v → v rdf:type C ---
	for _, r := range e.restrictions {
		if r.allValuesFrom == nil {
			continue
		}
		rk := term.TermKey(r.node)
		pk := term.TermKey(r.onProperty)

		for sk, types := range typeIndex {
			if _, ok := types[rk]; !ok {
				continue
			}
			objs := propObjs[spKey{sk, pk}]
			for _, obj := range objs {
				if oSubj, ok := obj.(term.Subject); ok {
					emit(oSubj, rdfType, r.allValuesFrom)
				}
			}
		}
	}

	// --- cls-maxc2: restriction maxCard 1, onProperty p, u rdf:type restriction,
	//     u p y1, u p y2 → y1 owl:sameAs y2 ---
	for _, r := range e.restrictions {
		if r.maxCard != 1 {
			continue
		}
		rk := term.TermKey(r.node)
		pk := term.TermKey(r.onProperty)

		for sk, types := range typeIndex {
			if _, ok := types[rk]; !ok {
				continue
			}
			objs := propObjs[spKey{sk, pk}]
			if len(objs) >= 2 {
				for i := 1; i < len(objs); i++ {
					s0, ok0 := objs[0].(term.Subject)
					si, oki := objs[i].(term.Subject)
					if ok0 && oki {
						emit(s0, sameAs, objs[i])
						emit(si, sameAs, objs[0])
					}
				}
			}
		}
	}

	// --- cls-maxqc3: restriction maxQualCard 1, onProperty p, onClass C,
	//     u rdf:type restriction, u p y1, y1 rdf:type C, u p y2, y2 rdf:type C → y1 sameAs y2 ---
	// --- cls-maxqc4: same but onClass owl:Thing (no type check needed) ---
	for _, r := range e.restrictions {
		if r.maxQualCard != 1 || r.onClass == nil {
			continue
		}
		rk := term.TermKey(r.node)
		pk := term.TermKey(r.onProperty)
		onClassKey := term.TermKey(r.onClass)
		isThing := onClassKey == term.TermKey(namespace.OWL.Thing)

		for sk, types := range typeIndex {
			if _, ok := types[rk]; !ok {
				continue
			}
			objs := propObjs[spKey{sk, pk}]
			// Filter objects that have the required class type
			var qualified []term.Term
			for _, obj := range objs {
				if isThing {
					qualified = append(qualified, obj)
				} else {
					oTypes := typeIndex[term.TermKey(obj)]
					if _, ok := oTypes[onClassKey]; ok {
						qualified = append(qualified, obj)
					}
				}
			}
			if len(qualified) >= 2 {
				for i := 1; i < len(qualified); i++ {
					s0, ok0 := qualified[0].(term.Subject)
					si, oki := qualified[i].(term.Subject)
					if ok0 && oki {
						emit(s0, sameAs, qualified[i])
						emit(si, sameAs, qualified[0])
					}
				}
			}
		}
	}

	// --- cls-int1: C intersectionOf (C1 ... Cn), y rdf:type C1, ..., y rdf:type Cn → y rdf:type C ---
	for ck, members := range e.intersectionOf {
		// Find all individuals that are typed as ALL member classes
		// Start with instances of first member, intersect with rest
		if len(members) == 0 {
			continue
		}
		// Collect candidates: all subjects with at least one member type
		candidates := make(map[string]term.Subject)
		for sk, types := range typeIndex {
			mk0 := term.TermKey(members[0])
			if _, ok := types[mk0]; ok {
				// Find the subject term
				for _, t := range allTriples {
					if term.TermKey(t.s) == sk && term.TermKey(t.p) == term.TermKey(rdfType) {
						candidates[sk] = t.s
						break
					}
				}
			}
		}
		// Filter to those that have ALL member types
		for sk, subj := range candidates {
			types := typeIndex[sk]
			allHave := true
			for _, m := range members {
				if _, ok := types[term.TermKey(m)]; !ok {
					allHave = false
					break
				}
			}
			if allHave {
				// Find the class term for ck
				for _, t := range allTriples {
					intOf := namespace.OWL.IntersectionOf
					if term.TermKey(t.p) == term.TermKey(intOf) && term.TermKey(t.s) == ck {
						emit(subj, rdfType, t.s)
						break
					}
				}
			}
		}
	}

	// --- cls-int2: C intersectionOf (C1 ... Cn), y rdf:type C → y rdf:type C1, ..., y rdf:type Cn ---
	for ck, members := range e.intersectionOf {
		for sk, types := range typeIndex {
			if _, ok := types[ck]; !ok {
				continue
			}
			for _, t := range allTriples {
				if term.TermKey(t.s) == sk && term.TermKey(t.p) == term.TermKey(rdfType) {
					for _, m := range members {
						emit(t.s, rdfType, m)
					}
					break
				}
			}
		}
	}

	// --- cls-uni: C unionOf (C1 ... Cn), y rdf:type Ci → y rdf:type C ---
	for ck, members := range e.unionOf {
		for sk, types := range typeIndex {
			for _, m := range members {
				if _, ok := types[term.TermKey(m)]; ok {
					// Find subject term
					for _, t := range allTriples {
						if term.TermKey(t.s) == sk && term.TermKey(t.p) == term.TermKey(rdfType) {
							// Find the union class term
							uOf := namespace.OWL.UnionOf
							for _, ut := range allTriples {
								if term.TermKey(ut.p) == term.TermKey(uOf) && term.TermKey(ut.s) == ck {
									emit(t.s, rdfType, ut.s)
									break
								}
							}
							break
						}
					}
					break // only need one matching member
				}
			}
		}
	}

	// --- cls-oo: C oneOf (a1 ... an) → a1 rdf:type C, ..., an rdf:type C ---
	for ck, members := range e.oneOfMembers {
		// Find the class term
		oof := namespace.OWL.OneOf
		for _, t := range allTriples {
			if term.TermKey(t.p) == term.TermKey(oof) && term.TermKey(t.s) == ck {
				for _, m := range members {
					if mSubj, ok := m.(term.Subject); ok {
						emit(mSubj, rdfType, t.s)
					}
				}
				break
			}
		}
	}
}
