package rdflibgo

// Store is the abstract interface for RDF triple storage backends.
// Ported from: rdflib.store.Store
type Store interface {
	// Add inserts a triple into the store, associated with the given context.
	Add(triple Triple, context Term)

	// AddN batch-adds quads (triple + context).
	AddN(quads []Quad)

	// Remove deletes triples matching the pattern from the given context.
	// If context is nil, removes from all contexts.
	Remove(pattern TriplePattern, context Term)

	// Triples returns an iterator over triples matching the pattern in the given context.
	// Each result includes the triple and its associated contexts.
	Triples(pattern TriplePattern, context Term) TripleIterator

	// Len returns the number of triples in the given context (nil = all).
	Len(context Term) int

	// Contexts returns an iterator over all contexts, optionally filtered by a triple.
	Contexts(triple *Triple) TermIterator

	// Bind associates a prefix with a namespace URI.
	Bind(prefix string, namespace URIRef)

	// Namespace returns the namespace URI for a prefix.
	Namespace(prefix string) (URIRef, bool)

	// Prefix returns the prefix for a namespace URI.
	Prefix(namespace URIRef) (string, bool)

	// Namespaces returns an iterator over all (prefix, namespace) bindings.
	Namespaces() NamespaceIterator

	// ContextAware reports whether this store supports named graphs.
	ContextAware() bool

	// TransactionAware reports whether this store supports transactions.
	TransactionAware() bool
}

// TripleIterator is a function that yields triples.
// Call it with a callback; return false from callback to stop.
type TripleIterator func(yield func(Triple) bool)

// TermIterator yields terms.
type TermIterator func(yield func(Term) bool)

// NamespaceIterator yields (prefix, namespace) pairs.
type NamespaceIterator func(yield func(string, URIRef) bool)
