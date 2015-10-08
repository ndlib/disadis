package main

import (
	"net/http"
)

// DsidMux multiplexes based on the datastream_id parameter.
// It does not look at the route or method. It does not perform
// any authentication.
//
// Takes a default handler and map[string]http.Handler.
// The default handler is used when the datastream_id parameter
// is missing. Otherwise a handler is looked up in by name.
// If no handler is associated with the value of datastream_id
// a 404 error is returned.
// In particular, the default handler is NOT used if datastream_id
// is provided, but does not match anything.
//
// The implementation is safe to be called by multiple goroutines.
type DsidMux struct {
	DefaultHandler http.Handler
	table          []routePair
}

type routePair struct {
	name string
	h    http.Handler
}

// AddHandler adds a (name, handler) pair to a DsidMux.
// If name has already been added, this will replace the old handler
// with h.
// Panics if h is nil.
func (dm *DsidMux) AddHandler(name string, h http.Handler) {
	if h == nil {
		panic("AddHandler passed nil handler")
	}
	for i := range dm.table {
		if dm.table[i].name == name {
			// duplicate name. Replace the old one
			dm.table[i].h = h
			return
		}
	}
	dm.table = append(dm.table, routePair{
		name: name,
		h:    h,
	})
}

func (dm *DsidMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	dsid := r.FormValue("datastream_id")
	if dsid == "" {
		if dm.DefaultHandler != nil {
			dm.DefaultHandler.ServeHTTP(w, r)
		} else {
			notFound(w)
		}
		return
	}
	for i := range dm.table {
		if dm.table[i].name == dsid {
			dm.table[i].h.ServeHTTP(w, r)
			return
		}
	}
	notFound(w)
}
