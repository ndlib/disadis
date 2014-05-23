package disseminator

import (
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/dbrower/disadis/fedora"
)

// Handles three types of routes
//
//	GET	/:id
//	GET	/:id?datastream_id=thumbnail
//	GET	/:id/:version
//
// The handler assumes that any authentication has already been performed.
// (See HydraAuth)
//
// Example Usage:
//	fedora := "http://fedoraAdmin:fedoraAdmin@localhost:8983/fedora/"
//	ha := NewHydraAuth(fedora, "vecnet:")
//	ha.Handler = NewVecnetDownloadHandler(NewRemoteFedora(fedora, "vecnet:"))
//	http.Handle("/d/", http.StripPrefix("/d/", ha))
//	return http.ListenAndServe(":"+port, nil)
type VecnetDownloadHandler struct {
	fedora fedora.Fedora
}

func NewVecnetDownloadHandler(f fedora.Fedora) http.Handler {
	return &VecnetDownloadHandler{
		fedora: f,
	}
}

func (vdh *VecnetDownloadHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Printf("%s %s", r.Method, r.URL.Path)

	if r.Method != "GET" {
		notFound(w)
		return
	}

	// "" / :id ( / :version )?
	path := strings.TrimPrefix(r.URL.Path, "/")
	path = strings.TrimSuffix(path, "/")
	components := strings.SplitN(path, "/", 2)

	var (
		pid    = components[0]
		dsname = "content"
		// -1 means most current version since 0 is a valid version number
		version int = -1
	)
	switch len(components) {
	case 1:
		// match /:id(?datastream_id=xxx)
		switch r.FormValue("datastream_id") {
		case "thumbnail":
			dsname = "thumbnail"
		case "", "content":
		default:
			notFound(w)
			return
		}
	case 2:
		// match /:id/:version
		version, err := strconv.Atoi(components[1])
		if err != nil || version < 0 {
			notFound(w)
			return
		}
	default:
		notFound(w)
		return
	}

	// see if the version requested matches the current version number
	if version >= 0 && version != vdh.currentVersion(pid, dsname) {
		http.Error(w, "403 Forbidden", http.StatusForbidden)
		return
	}

	content, info, err := vdh.fedora.GetDatastream(pid, dsname)
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

// returns -2 on error
func (vdh *VecnetDownloadHandler) currentVersion(pid string, dsname string) int {
	info, err := vdh.fedora.GetDatastreamInfo(pid, dsname)
	if err != nil {
		return -2
	}
	i := strings.LastIndex(info.VersionID, ".")
	if i == -1 {
		return -2
	}
	version, err := strconv.Atoi(info.VersionID[i+1 : len(info.VersionID)])
	if err != nil {
		return -2
	}
	return version
}
