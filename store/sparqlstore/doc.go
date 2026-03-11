// Package sparqlstore provides a store.Store implementation that communicates
// with a remote SPARQL endpoint over HTTP using the W3C SPARQL 1.1 Protocol.
//
// All methods are safe for concurrent use (delegated to net/http.Client).
//
// Reference: https://www.w3.org/TR/sparql11-protocol/
package sparqlstore
