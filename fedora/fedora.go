/*
Fedora provides a thin wrapper around the Fedora REST API.
It is not complete. Only methods needed by disadis have been
added.

Perhaps it might be advisable to make this its own package.
*/
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

var (
	FedoraNotFound      = errors.New("Item Not Found in Fedora")
	FedoraNotAuthorized = errors.New("Access Denied")
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

type ContentInfo struct {
	// These fields are from the headers in the fedora response
	// They may be empty strings, representing that the value is unknown
	Type        string
	Length      string
	Disposition string
}

// Create a reference to a remote Fedora instance.
// Pass in a complete URL including username and password, if necessary.
// For example
//	http://fedoraAdmin:password@localhost:8983/fedora/
// This reference does not buffer or cache Fedora responses.
// The namespace is of the form "temp:". It will be prefixed in front of
// all object identifiers.
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
	var path string = rf.hostpath + "objects/" + rf.namespace + id + "/datastreams/" + dsname + "/content"
	var info ContentInfo
	r, err := http.Get(path)
	if err != nil {
		return nil, info, err
	}
	if r.StatusCode != 200 {
		r.Body.Close()
		switch r.StatusCode {
		case 404:
			return nil, info, FedoraNotFound
		case 401:
			return nil, info, FedoraNotAuthorized
		default:
			return nil, info, fmt.Errorf("Received status %d from fedora", r.StatusCode)
		}
	}
	info.Type = r.Header.Get("Content-Type")
	info.Length = r.Header.Get("Content-Length")
	info.Disposition = r.Header.Get("Content-Disposition")
	return r.Body, info, nil
}

type DsInfo struct {
	Label     string `xml:"dsLabel"`
	VersionID string `xml:"dsVersionID"`
	State     string `xml:"dsState"`
	Checksum  string `xml:"dsChecksum"`
}

func (rf *remoteFedora) GetDatastreamInfo(id, dsname string) (DsInfo, error) {
	// TODO: make this joining smarter wrt not duplicating slashes
	var path string = rf.hostpath + "objects/" + rf.namespace + id + "/datastreams/" + dsname + "?format=xml"
	var info DsInfo
	r, err := http.Get(path)
	if err != nil {
		return info, err
	}
	if r.StatusCode != 200 {
		r.Body.Close()
		switch r.StatusCode {
		case 404:
			return info, FedoraNotFound
		case 401:
			return info, FedoraNotAuthorized
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

func NewTestFedora() *TestFedora {
	return &TestFedora{data: make(map[string][]byte)}
}

// TestFedora implements a simple in-memory Fedora stub which will return bytes which have
// already been specified by Set().
// Intended for testing. (Maybe move to a testing file?)
type TestFedora struct {
	data map[string][]byte
}

func (tf *TestFedora) GetDatastream(id, dsname string) (io.ReadCloser, ContentInfo, error) {
	ci := ContentInfo{}
	key := id + "/" + dsname
	v, ok := tf.data[key]
	if !ok {
		return nil, ci, fmt.Errorf("No such element %s", key)
	}
	ci.Type = "text/plain"
	ci.Length = fmt.Sprintf("%d", len(v))
	return ioutil.NopCloser(bytes.NewReader(v)), ci, nil
}

func (tf *TestFedora) GetDatastreamInfo(id, dsname string) (DsInfo, error) {
	key := id + "/" + dsname
	_, ok := tf.data[key]
	if !ok {
		return DsInfo{}, fmt.Errorf("No such element %s", key)
	}
	return DsInfo{
		Label:     "",
		VersionID: dsname + ".0",
		State:     "A",
		Checksum:  "",
	}, nil
}

func (tf *TestFedora) Set(id, dsname string, value []byte) {
	key := id + "/" + dsname
	tf.data[key] = value
}
