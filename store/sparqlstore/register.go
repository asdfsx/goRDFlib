package sparqlstore

import (
	"github.com/tggo/goRDFlib/plugin"
	"github.com/tggo/goRDFlib/store"
)

// init registers the "sparql" store type with the plugin registry,
// enabling discovery via plugin.GetStore("sparql").
func init() {
	plugin.RegisterStore("sparql", func() store.Store {
		return New("")
	})
}
