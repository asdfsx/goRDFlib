package reasoning

import "github.com/tggo/goRDFlib/term"

// unionFind implements a disjoint-set (union-find) data structure for owl:sameAs
// equivalence classes. Uses path compression and union-by-rank.
//
// Not safe for concurrent use.
type unionFind struct {
	parent map[string]string
	rank   map[string]int
	terms  map[string]term.Term // key → original term
}

func newUnionFind() *unionFind {
	return &unionFind{
		parent: make(map[string]string),
		rank:   make(map[string]int),
		terms:  make(map[string]term.Term),
	}
}

// size returns the number of elements in the union-find.
func (uf *unionFind) size() int {
	return len(uf.parent)
}

// add ensures the term exists in the union-find.
func (uf *unionFind) add(t term.Term) string {
	k := term.TermKey(t)
	if _, exists := uf.parent[k]; !exists {
		uf.parent[k] = k
		uf.rank[k] = 0
		uf.terms[k] = t
	}
	return k
}

// findKey returns the canonical representative key for the given key.
func (uf *unionFind) findKey(k string) string {
	if _, exists := uf.parent[k]; !exists {
		return k
	}
	// Path compression
	for uf.parent[k] != k {
		uf.parent[k] = uf.parent[uf.parent[k]]
		k = uf.parent[k]
	}
	return k
}

// find returns the canonical representative term for the given term.
// If the term is not in the UF, returns the term itself.
func (uf *unionFind) find(t term.Term) term.Term {
	k := term.TermKey(t)
	if _, exists := uf.parent[k]; !exists {
		return t
	}
	root := uf.findKey(k)
	return uf.terms[root]
}

// union merges the equivalence classes of two terms.
func (uf *unionFind) union(a, b term.Term) {
	ak := uf.add(a)
	bk := uf.add(b)
	rootA := uf.findKey(ak)
	rootB := uf.findKey(bk)
	if rootA == rootB {
		return
	}

	// Union by rank; on tie, pick lexicographically smaller key for determinism
	switch {
	case uf.rank[rootA] < uf.rank[rootB]:
		uf.parent[rootA] = rootB
	case uf.rank[rootA] > uf.rank[rootB]:
		uf.parent[rootB] = rootA
	default:
		// Same rank: pick lexicographically smaller as root for determinism
		if rootA < rootB {
			uf.parent[rootB] = rootA
			uf.rank[rootA]++
		} else {
			uf.parent[rootA] = rootB
			uf.rank[rootB]++
		}
	}
}

// classes returns all equivalence classes with more than one member.
// Each class is a slice of terms.
func (uf *unionFind) classes() [][]term.Term {
	groups := make(map[string][]term.Term, len(uf.parent)/2)
	for k := range uf.parent {
		root := uf.findKey(k)
		groups[root] = append(groups[root], uf.terms[k])
	}
	var result [][]term.Term
	for _, members := range groups {
		if len(members) > 1 {
			result = append(result, members)
		}
	}
	return result
}
