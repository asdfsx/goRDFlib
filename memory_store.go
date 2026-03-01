package rdflibgo

import "sync"

// MemoryStore is a thread-safe in-memory triple store with 3 indices (SPO, POS, OSP).
// Ported from: rdflib.plugins.stores.memory.SimpleMemory (non-context-aware variant)
type MemoryStore struct {
	mu sync.RWMutex

	// Triple indices: nested maps for efficient pattern matching.
	// Keys are N3() strings of terms for map-key compatibility.
	spo map[string]map[string]map[string]Triple // subject → predicate → object → triple
	pos map[string]map[string]map[string]Triple // predicate → object → subject → triple
	osp map[string]map[string]map[string]Triple // object → subject → predicate → triple

	// Namespace bindings
	nsPrefix map[string]URIRef // prefix → namespace
	nsURI    map[string]string // namespace → prefix

	count int
}

// NewMemoryStore creates a new empty in-memory store.
// Ported from: rdflib.plugins.stores.memory.SimpleMemory.__init__
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		spo:      make(map[string]map[string]map[string]Triple),
		pos:      make(map[string]map[string]map[string]Triple),
		osp:      make(map[string]map[string]map[string]Triple),
		nsPrefix: make(map[string]URIRef),
		nsURI:    make(map[string]string),
	}
}

func (m *MemoryStore) ContextAware() bool      { return false }
func (m *MemoryStore) TransactionAware() bool   { return false }

// Add inserts a triple into the store.
// Ported from: rdflib.plugins.stores.memory.SimpleMemory.add
func (m *MemoryStore) Add(t Triple, context Term) {
	m.mu.Lock()
	defer m.mu.Unlock()

	sk, pk, ok := termKey(t.Subject), termKey(t.Predicate), termKey(t.Object)

	// Check if already exists
	if po, exists := m.spo[sk]; exists {
		if o, exists := po[pk]; exists {
			if _, exists := o[ok]; exists {
				return // already present
			}
		}
	}

	// Insert into SPO
	if m.spo[sk] == nil {
		m.spo[sk] = make(map[string]map[string]Triple)
	}
	if m.spo[sk][pk] == nil {
		m.spo[sk][pk] = make(map[string]Triple)
	}
	m.spo[sk][pk][ok] = t

	// Insert into POS
	if m.pos[pk] == nil {
		m.pos[pk] = make(map[string]map[string]Triple)
	}
	if m.pos[pk][ok] == nil {
		m.pos[pk][ok] = make(map[string]Triple)
	}
	m.pos[pk][ok][sk] = t

	// Insert into OSP
	if m.osp[ok] == nil {
		m.osp[ok] = make(map[string]map[string]Triple)
	}
	if m.osp[ok][sk] == nil {
		m.osp[ok][sk] = make(map[string]Triple)
	}
	m.osp[ok][sk][pk] = t

	m.count++
}

// AddN batch-adds quads.
func (m *MemoryStore) AddN(quads []Quad) {
	for _, q := range quads {
		m.Add(q.Triple, q.Graph)
	}
}

// Remove deletes triples matching the pattern.
// Ported from: rdflib.plugins.stores.memory.SimpleMemory.remove
func (m *MemoryStore) Remove(pattern TriplePattern, context Term) {
	// Collect matches first, then delete (avoid modifying during iteration)
	var toRemove []Triple
	m.Triples(pattern, context)(func(t Triple) bool {
		toRemove = append(toRemove, t)
		return true
	})

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, t := range toRemove {
		sk, pk, ok := termKey(t.Subject), termKey(t.Predicate), termKey(t.Object)

		// Remove from SPO
		if po, exists := m.spo[sk]; exists {
			if o, exists := po[pk]; exists {
				delete(o, ok)
				if len(o) == 0 {
					delete(po, pk)
				}
			}
			if len(po) == 0 {
				delete(m.spo, sk)
			}
		}

		// Remove from POS
		if os, exists := m.pos[pk]; exists {
			if s, exists := os[ok]; exists {
				delete(s, sk)
				if len(s) == 0 {
					delete(os, ok)
				}
			}
			if len(os) == 0 {
				delete(m.pos, pk)
			}
		}

		// Remove from OSP
		if sp, exists := m.osp[ok]; exists {
			if p, exists := sp[sk]; exists {
				delete(p, pk)
				if len(p) == 0 {
					delete(sp, sk)
				}
			}
			if len(sp) == 0 {
				delete(m.osp, ok)
			}
		}

		m.count--
	}
}

// Triples returns matching triples.
// Ported from: rdflib.plugins.stores.memory.SimpleMemory.triples
func (m *MemoryStore) Triples(pattern TriplePattern, context Term) TripleIterator {
	return func(yield func(Triple) bool) {
		m.mu.RLock()
		defer m.mu.RUnlock()

		sk := optTermKey(pattern.Subject)
		pk := optPredKey(pattern.Predicate)
		ok := optTermKey(pattern.Object)

		switch {
		case sk != "" && pk != "" && ok != "":
			// Exact lookup
			if po, exists := m.spo[sk]; exists {
				if o, exists := po[pk]; exists {
					if t, exists := o[ok]; exists {
						yield(t)
					}
				}
			}

		case sk != "":
			// Subject bound → use SPO index
			if po, exists := m.spo[sk]; exists {
				for pk2, o := range po {
					if pk != "" && pk2 != pk {
						continue
					}
					for ok2, t := range o {
						if ok != "" && ok2 != ok {
							continue
						}
						if !yield(t) {
							return
						}
					}
				}
			}

		case pk != "":
			// Predicate bound → use POS index
			if os, exists := m.pos[pk]; exists {
				for ok2, s := range os {
					if ok != "" && ok2 != ok {
						continue
					}
					for sk2, t := range s {
						if sk != "" && sk2 != sk {
							continue
						}
						if !yield(t) {
							return
						}
					}
				}
			}

		case ok != "":
			// Object bound → use OSP index
			if sp, exists := m.osp[ok]; exists {
				for sk2, p := range sp {
					if sk != "" && sk2 != sk {
						continue
					}
					for pk2, t := range p {
						if pk != "" && pk2 != pk {
							continue
						}
						if !yield(t) {
							return
						}
					}
				}
			}

		default:
			// No constraints → iterate all from SPO
			for _, po := range m.spo {
				for _, o := range po {
					for _, t := range o {
						if !yield(t) {
							return
						}
					}
				}
			}
		}
	}
}

// Len returns the number of triples.
// Ported from: rdflib.plugins.stores.memory.SimpleMemory.__len__
func (m *MemoryStore) Len(context Term) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.count
}

// Contexts returns an empty iterator (not context-aware).
func (m *MemoryStore) Contexts(triple *Triple) TermIterator {
	return func(yield func(Term) bool) {}
}

// Bind associates a prefix with a namespace.
// Ported from: rdflib.plugins.stores.memory.SimpleMemory.bind
func (m *MemoryStore) Bind(prefix string, namespace URIRef) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nsPrefix[prefix] = namespace
	m.nsURI[namespace.Value()] = prefix
}

// Namespace returns the namespace URI for a prefix.
// Ported from: rdflib.plugins.stores.memory.SimpleMemory.namespace
func (m *MemoryStore) Namespace(prefix string) (URIRef, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ns, ok := m.nsPrefix[prefix]
	return ns, ok
}

// Prefix returns the prefix for a namespace URI.
// Ported from: rdflib.plugins.stores.memory.SimpleMemory.prefix
func (m *MemoryStore) Prefix(namespace URIRef) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.nsURI[namespace.Value()]
	return p, ok
}

// Namespaces returns an iterator over all namespace bindings.
// Ported from: rdflib.plugins.stores.memory.SimpleMemory.namespaces
func (m *MemoryStore) Namespaces() NamespaceIterator {
	return func(yield func(string, URIRef) bool) {
		m.mu.RLock()
		defer m.mu.RUnlock()
		for prefix, ns := range m.nsPrefix {
			if !yield(prefix, ns) {
				return
			}
		}
	}
}

// termKey returns a stable string key for a term (its N3 representation).
func termKey(t Term) string {
	return t.N3()
}

func optTermKey(t Term) string {
	if t == nil {
		return ""
	}
	return t.N3()
}

func optPredKey(p *URIRef) string {
	if p == nil {
		return ""
	}
	return p.N3()
}
