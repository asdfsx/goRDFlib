package badgerstore

import (
	"github.com/tggo/goRDFlib/plugin"
	"github.com/tggo/goRDFlib/store"
)

// init registers the "badger" store type with the plugin registry.
// The factory creates an in-memory BadgerStore by default, since directory
// paths cannot be passed through the plugin interface. For persistent storage,
// use New(WithDir(...)) directly.
func init() {
	plugin.RegisterStore("badger", func() store.Store {
		s, err := New(WithInMemory())
		if err != nil {
			panic("badgerstore: " + err.Error())
		}
		return s
	})
}
