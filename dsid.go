package main

import (
	"net/http"
)

// Multiplexes based on the datastream_id parameter.
// Does not look at the route or method at all.
// Does not perform any authentication.
//
// Takes a default handler and map[string]http.Handler.
// The default handler is used when the datastream_id parameter
// is missing. Otherwise a handler is looked up in by name.
// If no handler is associated with the value of datastream_id
// a 404 error is returned.
//
// The implementation does not use a map, and is safe to be
// called by multiple goroutines.
type DsidMux struct {
	h     http.Handler
	table []routePair
}

type routePair struct {
	name string
	h    http.Handler
}

func NewDsidMux(h http.Handler, table map[string]http.Handler) *DsidMux {
	var t []routePair
	for k, v := range table {
		t = append(t, routePair{
			name: k,
			h:    v,
		})
	}
	return &DsidMux{h: h, table: t}
}

func (dim *DsidMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	dsid := r.FormValue("datastream_id")
	if dsid == "" {
		dim.h.ServeHTTP(w, r)
		return
	}
	for i := range dim.table {
		if dim.table[i].name == dsid {
			dim.table[i].h.ServeHTTP(w, r)
			return
		}
	}
	notFound(w)
}
