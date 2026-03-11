package shacl

import (
	"fmt"
	"regexp"
)

// CheckWellformed checks that an SRL ruleset is well-formed.
//
// Well-formedness rules:
//  1. Every variable in the head must appear in a positive body triple pattern or be assigned by BIND
//  2. BIND target variable must not already be bound by preceding elements
//  3. FILTER variables must all be bound by preceding elements
func CheckWellformed(rs *RuleSet) error {
	for i, rule := range rs.Rules {
		if err := checkRuleWellformed(rule, i); err != nil {
			return err
		}
	}
	return nil
}

func checkRuleWellformed(rule SRLRule, idx int) error {
	// Process body elements sequentially — order matters for BIND/FILTER safety.
	bound := make(map[string]bool)
	for _, elem := range rule.Body {
		switch elem.Kind {
		case SRLBodyTriple:
			collectTripleVars(elem.Triple, bound)
		case SRLBodyBind:
			if bound[elem.BindVar] {
				return fmt.Errorf("rule %d: BIND variable ?%s already bound", idx, elem.BindVar)
			}
			// Check BIND expression references only bound variables
			for _, v := range extractExprVars(elem.BindExpr) {
				if !bound[v] {
					return fmt.Errorf("rule %d: BIND expression references unbound variable ?%s", idx, v)
				}
			}
			bound[elem.BindVar] = true
		case SRLBodyFilter:
			for _, v := range extractExprVars(elem.FilterExpr) {
				if !bound[v] {
					return fmt.Errorf("rule %d: FILTER references unbound variable ?%s", idx, v)
				}
			}
		case SRLBodyNot:
			// NOT does not bind variables in the outer scope.
			// Variables referenced in NOT should be bound by preceding elements.
			// (New variables inside NOT are local.)
		}
	}

	// Check that all head variables are bound (positive triples or BIND).
	for _, t := range rule.Head {
		for _, v := range srlTermVars(t.Subject) {
			if !bound[v] {
				return fmt.Errorf("rule %d: head variable ?%s not bound in body", idx, v)
			}
		}
		for _, v := range srlTermVars(t.Predicate) {
			if !bound[v] {
				return fmt.Errorf("rule %d: head variable ?%s not bound in body", idx, v)
			}
		}
		for _, v := range srlTermVars(t.Object) {
			if !bound[v] {
				return fmt.Errorf("rule %d: head variable ?%s not bound in body", idx, v)
			}
		}
	}
	return nil
}

func collectTripleVars(t SRLTriple, bound map[string]bool) {
	for _, v := range srlTermVars(t.Subject) {
		bound[v] = true
	}
	for _, v := range srlTermVars(t.Predicate) {
		bound[v] = true
	}
	for _, v := range srlTermVars(t.Object) {
		bound[v] = true
	}
}

func srlTermVars(t SRLTerm) []string {
	switch t.Kind {
	case SRLTermVariable:
		return []string{t.Value}
	case SRLTermTripleTerm:
		var vars []string
		if t.TTSubject != nil {
			vars = append(vars, srlTermVars(*t.TTSubject)...)
		}
		if t.TTPredicate != nil {
			vars = append(vars, srlTermVars(*t.TTPredicate)...)
		}
		if t.TTObject != nil {
			vars = append(vars, srlTermVars(*t.TTObject)...)
		}
		return vars
	}
	return nil
}

var varPattern = regexp.MustCompile(`\?([A-Za-z_]\w*)`)

func extractExprVars(expr string) []string {
	matches := varPattern.FindAllStringSubmatch(expr, -1)
	seen := make(map[string]bool, len(matches))
	var vars []string
	for _, m := range matches {
		name := m[1]
		if !seen[name] {
			seen[name] = true
			vars = append(vars, name)
		}
	}
	return vars
}
