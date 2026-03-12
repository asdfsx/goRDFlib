// Package badgerstore provides a persistent store.Store implementation backed
// by Badger (dgraph-io/badger/v4), an LSM-tree key-value engine.
//
// Triples are indexed using three key prefixes (SPO, POS, OSP) for efficient
// pattern matching via prefix scans. Named graphs are supported by embedding
// the graph key into every index key.
//
// All methods are safe for concurrent use (delegated to Badger's MVCC).
//
// Reference: https://dgraph.io/docs/badger/
package badgerstore
