// Package fedora provides a thin wrapper around the Fedora REST API.
// It is not complete. Only the methods needed by disadis are present.
package fedora

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
)

// Exported errors
var (
	ErrNotFound      = errors.New("Item Not Found in Fedora")
	ErrNotAuthorized = errors.New("Access Denied")
)

// Fedora represents a Fedora Commons server. The exact nature of the
// server is unspecified.
type Fedora interface {
	// Return the contents of the dsname datastream of object id.
	// You are expected to close it when you are finished.
	GetDatastream(id, dsname string) (io.ReadCloser, ContentInfo, error)
	// GetDatastreamInfo returns the metadata Fedora stores about the named
	// datastream.
	GetDatastreamInfo(id, dsname string) (DsInfo, error)
}

// ContentInfo holds the most basic metadata about a datastream.
type ContentInfo struct {
	// These fields are from the headers in the fedora response
	// They may be empty strings, representing that the value is unknown
	Type        string
	Length      string
	Disposition string
}

// NewRemote creates a reference to a remote Fedora repository.
// fedoraPath is a complete URL including username and password, if necessary.
// For example
//	http://fedoraAdmin:password@localhost:8983/fedora/
// The namespace is expected to have the form "temp:", and it will be prefixed
// to all object identifiers.
// The returned structure does not buffer or cache Fedora responses.
func NewRemote(fedoraPath string, namespace string) Fedora {
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
func (rf *remoteFedora) GetDatastream(id, dsname string) (io.ReadCloser, ContentInfo, error) {
	// TODO: make this joining smarter wrt not duplicating slashes
	var path = rf.hostpath + "objects/" + rf.namespace + id + "/datastreams/" + dsname + "/content"
	var info ContentInfo
	r, err := http.Get(path)
	if err != nil {
		return nil, info, err
	}
	if r.StatusCode != 200 {
		r.Body.Close()
		switch r.StatusCode {
		case 404:
			return nil, info, ErrNotFound
		case 401:
			return nil, info, ErrNotAuthorized
		default:
			return nil, info, fmt.Errorf("Received status %d from fedora", r.StatusCode)
		}
	}
	// if fedora had an R datastream then these headers are comming from
	// wherever fedora redirected us, and NOT from fedora.
	info.Type = r.Header.Get("Content-Type")
	info.Length = r.Header.Get("Content-Length")
	info.Disposition = r.Header.Get("Content-Disposition")
	return r.Body, info, nil
}

// DsInfo holds more complete metadata on a datastream (as opposed to the
// ContentInfo structure)
type DsInfo struct {
	Label     string `xml:"dsLabel"`
	VersionID string `xml:"dsVersionID"`
	State     string `xml:"dsState"`
	Checksum  string `xml:"dsChecksum"`
}

func (rf *remoteFedora) GetDatastreamInfo(id, dsname string) (DsInfo, error) {
	// TODO: make this joining smarter wrt not duplicating slashes
	var path = rf.hostpath + "objects/" + rf.namespace + id + "/datastreams/" + dsname + "?format=xml"
	var info DsInfo
	r, err := http.Get(path)
	if err != nil {
		return info, err
	}
	if r.StatusCode != 200 {
		r.Body.Close()
		switch r.StatusCode {
		case 404:
			return info, ErrNotFound
		case 401:
			return info, ErrNotAuthorized
		default:
			return info, fmt.Errorf("Received status %d from fedora", r.StatusCode)
		}
	}
	dec := xml.NewDecoder(r.Body)
	err = dec.Decode(&info)
	r.Body.Close()
	return info, err
}

// Version returns the version number as an integer.
// For example, if VersionID is "content.2" Version() will
// return 2. It returns -1 on error.
func (info DsInfo) Version() int {
	// VersionID has the form "something.X"
	i := strings.LastIndex(info.VersionID, ".")
	if i == -1 {
		//log.Println("Error parsing", info.VersionID)
		return -1
	}
	version, err := strconv.Atoi(info.VersionID[i+1:])
	if err != nil {
		//log.Println(err)
		return -1
	}
	return version
}

// NewTestFedora creates an empty TestFedora object.
func NewTestFedora() *TestFedora {
	return &TestFedora{data: make(map[string][]byte)}
}

// TestFedora implements a simple in-memory Fedora stub which will return bytes which have
// already been specified by Set().
// Intended for testing. (Maybe move to a testing file?)
type TestFedora struct {
	data map[string][]byte
}

// GetDatastream returns a ReadCloser which holds the content of the named
// datastream on the given fedora object.
func (tf *TestFedora) GetDatastream(id, dsname string) (io.ReadCloser, ContentInfo, error) {
	ci := ContentInfo{}
	key := id + "/" + dsname
	v, ok := tf.data[key]
	if !ok {
		return nil, ci, ErrNotFound
	}
	ci.Type = "text/plain"
	ci.Length = fmt.Sprintf("%d", len(v))
	return ioutil.NopCloser(bytes.NewReader(v)), ci, nil
}

// GetDatastreamInfo returns Fedora's metadata for the given datastream.
func (tf *TestFedora) GetDatastreamInfo(id, dsname string) (DsInfo, error) {
	key := id + "/" + dsname
	_, ok := tf.data[key]
	if !ok {
		return DsInfo{}, ErrNotFound
	}
	return DsInfo{
		Label:     "",
		VersionID: dsname + ".0",
		State:     "A",
		Checksum:  "",
	}, nil
}

// Set the given datastream to have the given content.
func (tf *TestFedora) Set(id, dsname string, value []byte) {
	key := id + "/" + dsname
	tf.data[key] = value
}
