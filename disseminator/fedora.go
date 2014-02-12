package disseminator

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
)

type Fedora interface {
	GetDatastream(id, dsname string) (io.ReadCloser, error)
}

//
func NewRemoteFedora(fedoraPath string) Fedora {
	rf := &remoteFedora{hostpath: fedoraPath}
	if rf.hostpath[len(rf.hostpath)] != '/' {
		rf.hostpath = rf.hostpath + "/"
	}
	return rf
}

type remoteFedora struct {
	hostpath string
}

// returns the contents of the datastream `dsname`.
// The returned stream needs to be closed when finished.
func (rf *remoteFedora) GetDatastream(id, dsname string) (io.ReadCloser, error) {
	var path string = rf.hostpath + id + "/datastreams/" + dsname + "/content"
	r, err := http.Get(path)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	if r.StatusCode != 200 {
		log.Printf("Got status %d from fedora", r.StatusCode)
		return nil, err
	}

	return r.Body, nil
}

func newTestFedora() *TestFedora {
	return &TestFedora{data: make(map[string][]byte)}
}

// a simple fedora implementation which will return bytes which have
// already been specified by Set().
// Intended for testing. (Maybe move to a testing file?)
type TestFedora struct {
	data map[string][]byte
}

func (tf *TestFedora) GetDatastream(id, dsname string) (io.ReadCloser, error) {
	key := id + "/" + dsname
	v, ok := tf.data[key]
	if !ok {
		return nil, fmt.Errorf("No such element %s/%s", id, dsname)
	}
	return ioutil.NopCloser(bytes.NewReader(v)), nil
}

func (tf *TestFedora) Set(id, dsname string, value []byte) {
	key := id + "/" + dsname
	tf.data[key] = value
}
