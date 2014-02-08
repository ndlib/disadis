package disseminator

import (
	"log"
	"net/http"
	"strings"
)

// Setup the HTTP handlers and run.
// Uses the port which is passed in as a string.
// The HTTP handlers will log requests to the default log given by the
// standard log library.
// This function only returns if there was an error, which is returned.
func Run(port string) error {
	fedora := "http://fedoraAdmin:fedoraAdmin@localhost:8983/fedora/"
	dh := NewDownloadHandler(nil,
		NewHydraAuth(fedora, "vecnet:"),
		NewFedoraSource(fedora, "vecnet:"))
	http.Handle("/download/", dh)
	return http.ListenAndServe(":"+port, nil)
}

// Handle two types of routes
//
//	GET	/download/:id
//	GET	/download/:id/thumbnail
//
// The id is first checked against a pattern to see if it is even remotely valid.
// If so, the user is decoded from the request, and we check the access rights
// to the object.
//
// We could handle a more generic route of
//
//	/download/:id/:datastream
//
// but that would require some blacklisting or whitelisting of datastream names.
type downloadHandler struct {
	auth    Auth
	source  Source
	idcheck IdChecker
}

type IdChecker func(id string) bool

func NewDownloadHandler(id IdChecker, auth Auth, s Source) http.Handler {
	return &downloadHandler{
		auth:    auth,
		source:  s,
		idcheck: id,
	}
}

func (dh *downloadHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var (
		isThumb    bool
		path       string
		components []string
	)

	log.Println("Start")

	if r.Method != "GET" {
		goto notfound
	}

	// "" / "downloads" / "id" ( / thumbnail )?

	path = strings.TrimPrefix(r.URL.Path, "/d/")
	path = strings.TrimSuffix(path, "/")
	components = strings.SplitN(path, "/", 2)

	switch {
	case len(components) > 2 || len(components) == 0:
		goto notfound
	case len(components) == 2:
		if components[1] != "thumbnail" {
			goto notfound
		}
		isThumb = true
	}

	if dh.idcheck != nil && !dh.idcheck(components[0]) {
		goto notfound
	}

	if !dh.auth.Check(r, components[0], isThumb) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	dh.source.Get(w, components[0], isThumb)
	log.Println("End")
	return

notfound:
	http.Error(w, "404 Not Found", http.StatusNotFound)
	return
}

func isCurateId(s string) bool { return scanId(s, "eeddeeddede") }
func isVecnetId(s string) bool { return scanId(s, "eeddeedde") }

const (
	noidx string = "0123456789bcdfghjkmnpqrstvwxz"
)

// Compare an id against template character by character.
// An 'd' in template must match with a digit in id.
// An 'e' in template must match with a noid alphanumeric character in id,
// which consist of "0123456789bcdfghjkmnpqrstvwxz"
// returns false if any match fails, Otherwise returns true
func scanId(id, template string) bool {
	if len(id) != len(template) {
		return false
	}

	for i := range id {
		var allowed string = "0123456789"
		if template[i] == 'e' {
			allowed = noidx
		}
		if strings.IndexByte(allowed, id[i]) < 0 {
			return false
		}
	}
	return true
}
