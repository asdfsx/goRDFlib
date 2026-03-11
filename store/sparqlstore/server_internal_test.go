package sparqlstore

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tggo/goRDFlib/sparql"
)

func TestWriteConstructResultNilGraph(t *testing.T) {
	srv := &Server{}
	w := httptest.NewRecorder()
	result := &sparql.Result{Type: "CONSTRUCT", Graph: nil}
	srv.writeConstructResult(w, result, "")

	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "n-triples") {
		t.Errorf("content type = %s, want n-triples", ct)
	}
	// Body should be empty since graph is nil
	if w.Body.Len() != 0 {
		t.Errorf("body should be empty for nil graph, got %d bytes", w.Body.Len())
	}
}
