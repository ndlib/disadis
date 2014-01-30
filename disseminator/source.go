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

type FedoraSource struct {
	Url            string
	User, Password string
	Ns             string
}

func (fs *FedoraSource) Get(w http.ResponseWriter, id string, isThumb bool) {
	var path string

	if isThumb {
		path = fs.Url + "objects/" + fs.Ns + id + "/datastreams/thumbnail/content"
	} else {
		path = fs.Url + "objects/" + fs.Ns + id + "/datastreams/content/content"
	}

	log.Printf("asking Fedora %s", path)
	r, err := http.Get(path)

	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	if r.StatusCode != 200 {
		log.Printf("Got status code %d from fedora\n", r.StatusCode)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	//w.Header().Set("Content-Type",

	io.Copy(w, r.Body)

	// maybe use http.ServeContent()?
}
