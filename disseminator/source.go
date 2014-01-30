package disseminator

import (
	"fmt"
	"io"
	"log"
	"net/http"
)

type Source interface {
	Get(w http.ResponseWriter, id string, isThumb bool)
}

type echoSource struct{}

func NewEchoSource() Source {
	return &echoSource{}
}

func (es *echoSource) Get(w http.ResponseWriter, id string, isThumb bool) {
	fmt.Fprintf(w, "Echo\nid = %s\nisthumb = %v\n", id, isThumb)
}

type fedoraSource struct {
	cachedPrefix string
}

func NewFedoraSource(url, namespace string) *fedoraSource {
	return &fedoraSource{
		cachedPrefix: url + "objects/" + namespace,
	}
}

func (fs *fedoraSource) Get(w http.ResponseWriter, id string, isThumb bool) {
	var path string

	if isThumb {
		path = fs.cachedPrefix + id + "/datastreams/thumbnail/content"
	} else {
		path = fs.cachedPrefix + id + "/datastreams/content/content"
	}

	log.Printf("asking Fedora %s", path)
	r, err := http.Get(path)
	if err != nil {
		log.Println(err)
		http.Error(w, "500 Internal Error", http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	if r.StatusCode != 200 {
		log.Printf("Got status %d from fedora\n", r.StatusCode)
		if r.StatusCode == 404 {
			http.Error(w, "404 Not Found", http.StatusNotFound)
		} else {
			http.Error(w, "500 Internal Error", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", r.Header.Get("Content-Type"))
	w.Header().Set("Content-Length", r.Header.Get("Content-Length"))

	io.Copy(w, r.Body)

	// maybe use http.ServeContent()?
}
