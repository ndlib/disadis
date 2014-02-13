package disseminator

import (
	"io"
	"log"
	"net/http"
	"strings"
)

// Handle two types of routes
//
//	GET	/:id
//	GET	/:id/thumbnail
//
// We could handle a more generic route of
//
//	/download/:id/:datastream
//
// but that would require some blacklisting or whitelisting of datastream names.
//
// The handler assumes that any authentication has already been performed.
// (See HydraAuth)
//
// Example Usage:
//	fedora := "http://fedoraAdmin:fedoraAdmin@localhost:8983/fedora/"
//	ha := NewHydraAuth(fedora, "vecnet:")
//	ha.Handler = NewDownloadHandler(NewRemoteFedora(fedora, "vecnet:"))
//	http.Handle("/d/", http.StripPrefix("/d/", ha))
//	return http.ListenAndServe(":"+port, nil)
type DownloadHandler struct {
	fedora Fedora
}

func NewDownloadHandler(f Fedora) http.Handler {
	return &DownloadHandler{
		fedora: f,
	}
}

func (dh *DownloadHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var (
		path       string
		components []string
		dsname     string = "contents"
		content    io.ReadCloser
		err        error
	)

	log.Println("Start")

	if r.Method != "GET" {
		goto notfound
	}

	// "" / "id" ( / thumbnail )?

	path = strings.TrimPrefix(r.URL.Path, "/")
	path = strings.TrimSuffix(path, "/")
	components = strings.SplitN(path, "/", 2)

	switch {
	case len(components) > 2 || len(components) == 0:
		goto notfound
	case len(components) == 2:
		if components[1] != "thumbnail" {
			goto notfound
		}
		dsname = "thumbnail"
	}

	content, err = dh.fedora.GetDatastream(components[0], dsname)
	if err != nil {
		switch err {
		case FedoraNotFound:
			goto notfound
		default:
			log.Printf("Got fedora error: %s", err)
			http.Error(w, "500 Internal Error", http.StatusInternalServerError)
			return
		}
	}
	defer content.Close()

	//dh.source.Get(w, components[0], isThumb)
	io.Copy(w, content)
	log.Println("End")
	return

notfound:
	http.Error(w, "404 Not Found", http.StatusNotFound)
	return
}
