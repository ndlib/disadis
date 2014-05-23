package disseminator

import (
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/dbrower/disadis/fedora"
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
	fedora fedora.Fedora
}

func NewDownloadHandler(f fedora.Fedora) http.Handler {
	return &DownloadHandler{
		fedora: f,
	}
}

func notFound(w http.ResponseWriter) {
	http.Error(w, "404 Not Found", http.StatusNotFound)
}

func (dh *DownloadHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Printf("%s %s", r.Method, r.URL.Path)

	if r.Method != "GET" {
		notFound(w)
		return
	}

	// "" / "id" ( / thumbnail )?
	path := strings.TrimPrefix(r.URL.Path, "/")
	path = strings.TrimSuffix(path, "/")
	components := strings.SplitN(path, "/", 2)

	var dsname = "content"
	switch len(components) {
	case 1:
		/* nothing to be done */
	case 2:
		if components[1] != "thumbnail" {
			notFound(w)
			return
		}
		dsname = "thumbnail"
	default:
		notFound(w)
		return
	}

	content, info, err := dh.fedora.GetDatastream(components[0], dsname)
	if err != nil {
		switch err {
		case fedora.FedoraNotFound:
			notFound(w)
			return
		default:
			log.Printf("Got fedora error: %s", err)
			http.Error(w, "500 Internal Error", http.StatusInternalServerError)
			return
		}
	}
	defer content.Close()

	w.Header().Set("Content-Type", info.Type)
	w.Header().Set("Content-Length", info.Length)
	w.Header().Set("Content-Disposition", info.Disposition)
	w.Header().Set("Content-Transfer-Encoding", "binary")
	w.Header().Set("Cache-Control", "private")

	io.Copy(w, content)
	log.Println("End")
	return
}
