package disseminator

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
)

var (
	FedoraNotFound      = errors.New("Item Not Found in Fedora")
	FedoraNotAuthorized = errors.New("Access Denied")
)

// Fedora represents a Fedora Commons server. The exact nature of the
// server is unspecified.
type Fedora interface {
	// Return the contents of the dsname datastream of object id.
	// You are expected to close it when you are finished.
	GetDatastream(id, dsname string) (io.ReadCloser, error)
}

// Create a reference to a remote Fedora instance.
// Pass in a complete URL including username and password, if necessary.
// For example
//	http://fedoraAdmin:password@localhost:8983/fedora/
// This reference does not buffer or cache Fedora responses.
// The namespace is of the form "temp:". It will be prefixed in front of
// all object identifiers.
func NewRemoteFedora(fedoraPath string, namespace string) Fedora {
	rf := &remoteFedora{hostpath: fedoraPath, namespace: namespace}
	if rf.hostpath[len(rf.hostpath)-1] != '/' {
		rf.hostpath = rf.hostpath + "/"
	}
	return rf
}

type remoteFedora struct {
	hostpath  string
	namespace string
}

// returns the contents of the datastream `dsname`.
// The returned stream needs to be closed when finished.
func (rf *remoteFedora) GetDatastream(id, dsname string) (io.ReadCloser, error) {
	// TODO: make this joining smarter wrt not duplicating slashes
	var path string = rf.hostpath + "objects/" + rf.namespace + id + "/datastreams/" + dsname + "/content"
	r, err := http.Get(path)
	if err != nil {
		return nil, err
	}
	if r.StatusCode != 200 {
		r.Body.Close()
		switch r.StatusCode {
		case 404:
			return nil, FedoraNotFound
		case 401:
			return nil, FedoraNotAuthorized
		default:
			return nil, fmt.Errorf("Got status %d from fedora", r.StatusCode)
		}
	}
	return r.Body, nil
}

func newTestFedora() *TestFedora {
	return &TestFedora{data: make(map[string][]byte)}
}

// TestFedora implements a simple in-memory Fedora stub which will return bytes which have
// already been specified by Set().
// Intended for testing. (Maybe move to a testing file?)
type TestFedora struct {
	data map[string][]byte
}

func (tf *TestFedora) GetDatastream(id, dsname string) (io.ReadCloser, error) {
	key := id + "/" + dsname
	v, ok := tf.data[key]
	if !ok {
		return nil, fmt.Errorf("No such element %s", key)
	}
	return ioutil.NopCloser(bytes.NewReader(v)), nil
}

func (tf *TestFedora) Set(id, dsname string, value []byte) {
	key := id + "/" + dsname
	tf.data[key] = value
}
