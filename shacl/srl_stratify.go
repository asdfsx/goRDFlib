package shacl

import "fmt"

// Stratify computes a stratification of the rules in the RuleSet.
// It returns strata (groups of rule indices that can be evaluated together)
// or an error if the rules contain a negative cycle (unstratifiable).
func Stratify(rs *RuleSet) ([][]int, error) {
	if len(rs.Rules) == 0 {
		return nil, nil
	}

	n := len(rs.Rules)

	// Build rule-level dependency graph.
	// Rule B depends positively on Rule A if A's head predicate appears in B's positive body.
	// Rule B depends negatively on Rule A if A's head could match a pattern in B's NOT body.
	type edge struct {
		to       int
		negative bool
	}
	deps := make([][]edge, n)

	for b := 0; b < n; b++ {
		posBodyPats := collectBodyPatterns(rs.Rules[b].Body, false)
		negBodyPats := collectBodyPatterns(rs.Rules[b].Body, true)

		for a := 0; a < n; a++ {
			headPats := rs.Rules[a].Head

			// Check positive dependency.
			if patternsOverlap(headPats, posBodyPats) {
				deps[b] = append(deps[b], edge{to: a, negative: false})
			}

			// Check negative dependency: A's head could match B's NOT patterns.
			if patternsCouldMatch(headPats, negBodyPats) {
				deps[b] = append(deps[b], edge{to: a, negative: true})
			}
		}
	}

	// Assign strata using iterative refinement.
	strata := make([]int, n)
	maxIter := n + 1
	for iter := 0; iter < maxIter; iter++ {
		changed := false
		for b := 0; b < n; b++ {
			for _, e := range deps[b] {
				required := strata[e.to]
				if e.negative {
					required++
				}
				if strata[b] < required {
					strata[b] = required
					changed = true
				}
			}
		}
		if !changed {
			break
		}
		for _, s := range strata {
			if s > n {
				return nil, fmt.Errorf("ruleset is not stratifiable: negative cycle detected")
			}
		}
	}

	// Verify no negative dependency within or backward.
	for b := 0; b < n; b++ {
		for _, e := range deps[b] {
			if e.negative && strata[b] <= strata[e.to] {
				return nil, fmt.Errorf("ruleset is not stratifiable: negative cycle detected")
			}
		}
	}

	// Group rules by stratum.
	maxStratum := 0
	for _, s := range strata {
		if s > maxStratum {
			maxStratum = s
		}
	}
	result := make([][]int, maxStratum+1)
	for i, s := range strata {
		result[s] = append(result[s], i)
	}
	return result, nil
}

// collectBodyPatterns collects triple patterns from body elements.
// If negativeOnly, only patterns inside NOT blocks are returned.
func collectBodyPatterns(elements []SRLBodyElement, negativeOnly bool) []SRLTriple {
	var pats []SRLTriple
	for _, elem := range elements {
		switch elem.Kind {
		case SRLBodyTriple:
			if !negativeOnly {
				pats = append(pats, elem.Triple)
			}
		case SRLBodyNot:
			if negativeOnly {
				for _, ne := range elem.NotBody {
					if ne.Kind == SRLBodyTriple {
						pats = append(pats, ne.Triple)
					}
				}
			}
		}
	}
	return pats
}

// patternsOverlap checks if any head pattern shares a predicate with any body pattern.
func patternsOverlap(head, body []SRLTriple) bool {
	for _, h := range head {
		hp := extractPredIRI(h)
		if hp == "" {
			continue
		}
		for _, b := range body {
			bp := extractPredIRI(b)
			if hp == bp || hp == "?var" || bp == "?var" {
				return true
			}
		}
	}
	return false
}

// patternsCouldMatch checks if any head pattern could produce a triple that
// matches a NOT body pattern. Two patterns "could match" if they share the
// same predicate and have no conflicting constants in corresponding positions.
func patternsCouldMatch(head, negBody []SRLTriple) bool {
	for _, h := range head {
		for _, nb := range negBody {
			if triplePatternCouldMatch(h, nb) {
				return true
			}
		}
	}
	return false
}

func triplePatternCouldMatch(head, negPat SRLTriple) bool {
	return termCouldMatch(head.Subject, negPat.Subject) &&
		termCouldMatch(head.Predicate, negPat.Predicate) &&
		termCouldMatch(head.Object, negPat.Object)
}

// termCouldMatch returns true if two terms could potentially unify.
// Variables match anything. Two constants match only if equal.
func termCouldMatch(a, b SRLTerm) bool {
	if a.Kind == SRLTermVariable || b.Kind == SRLTermVariable {
		return true
	}
	// Both are constants — must be equal.
	return srlTermToKey(a) == srlTermToKey(b)
}

func extractPredIRI(t SRLTriple) string {
	switch t.Predicate.Kind {
	case SRLTermIRI:
		return t.Predicate.Value
	case SRLTermVariable:
		return "?var"
	}
	return ""
}
