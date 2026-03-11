package reasoning

import "github.com/tggo/goRDFlib/term"

// dedupSet tracks triples that have already been seen/added, preventing duplicates.
type dedupSet struct {
	existing map[string]struct{}
}

func newDedupSet(capacity int) *dedupSet {
	return &dedupSet{existing: make(map[string]struct{}, capacity)}
}

// addNew checks if a triple is new and marks it as existing. Returns true if new.
func (d *dedupSet) addNew(s term.Subject, p term.URIRef, o term.Term) bool {
	k := tripleKey(s, p, o)
	if _, exists := d.existing[k]; exists {
		return false
	}
	d.existing[k] = struct{}{}
	return true
}

func tripleKey(s term.Subject, p term.URIRef, o term.Term) string {
	return term.TermKey(s) + "|" + term.TermKey(p) + "|" + term.TermKey(o)
}
