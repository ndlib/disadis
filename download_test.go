package main

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ndlib/disadis/fedora"
)

func TestDownload(t *testing.T) {
	ts := setupHandler()
	defer ts.Close()

	var sequence = []struct {
		verb, route string
		status      int
		expected    string
	}{
		// Test pool list and creation
		{"GET", "/0123", 200, "hello"},
		{"HEAD", "/0123", 200, ""},
		{"GET", "/0123/0", 200, "hello"},
		{"GET", "/0123/1", 403, ""},
		{"HEAD", "/0123/0", 200, ""},

		{"GET", "/123", 200, "goodbye"},
		{"HEAD", "/123", 200, ""},
		{"GET", "/123/0", 200, "goodbye"},
		{"GET", "/123/1", 403, ""},
		{"HEAD", "/123/0", 200, ""},

		{"GET", "/0123?datastream_id=content", 200, "hello"},
		{"POST", "/0123", 404, ""},

		{"GET", "/badsize", 200, "hola"},

		// It applies the correct prefix
		{"GET", "/xyz", 404, ""},
		{"HEAD", "/xyz", 404, ""},
		{"GET", "/xyz/0", 404, ""},
		{"GET", "/xyz/1", 404, ""},
		{"HEAD", "/xyz/0", 404, ""},

		// identifiers are assumed to not have more than 64 characters
		{"GET", "/123456789012345678901234567890123456789012345678901234567890", 404, ""},
	}
	for _, s := range sequence {
		checkRoute(t, s.verb, ts.URL+s.route, s.status, s.expected)
	}
}

// See if the returned content type is pulled from the datastream metadata and not
// from the returned Content-Type. (DLTP-568)
func TestDLTP568(t *testing.T) {
	ts := setupHandler()
	defer ts.Close()

	table := []struct {
		verb, route, contenttype string
	}{
		{"GET", "/redirect", "audio/mpeg"},
		{"HEAD", "/redirect", "audio/mpeg"},
		{"GET", "/0123", ""},
		{"HEAD", "/0123", ""},
	}
	for _, s := range table {
		checkContentType(t, s.verb, ts.URL+s.route, 200, s.contenttype)
	}
}

func checkContentType(t *testing.T, verb, route string, status int, expectedType string) {
	r, _ := checkRouteX(t, verb, route, status, "", nil)
	recvType := r.Header.Get("Content-Type")
	if recvType != expectedType {
		t.Errorf("%s: Expected %s, Received %s", route, expectedType, recvType)
	}
}

func checkRoute(t *testing.T, verb, route string, status int, expected string) {
	checkRouteX(t, verb, route, status, expected, nil)
}

func checkRouteX(t *testing.T, verb, route string, status int, expected string, setup func(*http.Request)) (*http.Response, []byte) {
	req, err := http.NewRequest(verb, route, nil)
	if err != nil {
		t.Fatal("Problem creating request", err)
	}
	if setup != nil {
		setup(req)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(route, err)
	}
	if resp.StatusCode != status {
		t.Errorf("%s: Expected status %d and received %d",
			route,
			status,
			resp.StatusCode)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(route, err)
	}
	if expected != "" {
		if string(body) != expected {
			t.Errorf("%s: Expected body %s, got %v",
				route,
				expected,
				body)
		}
	}
	resp.Body.Close()
	return resp, body
}

func TestRangeRequest(t *testing.T) {
	ts := setupHandler()
	defer ts.Close()

	checkRouteX(t, "GET", ts.URL+"/abc", 206, "longer", func(req *http.Request) {
		req.Header.Add("Range", "bytes=2-7")
	})
	checkRouteX(t, "GET", ts.URL+"/abc", 206, "longer string", func(req *http.Request) {
		req.Header.Add("Range", "bytes=2-")
	})
	checkRouteX(t, "GET", ts.URL+"/abc", 206, "", func(req *http.Request) {
		req.Header.Add("Range", "bytes=2-7,10-")
	})
}

// setupHandler returns a test server seeded with some content.
func setupHandler() *httptest.Server {
	tf := fedora.NewTestFedora()
	tf.Set("test:0123", "content", fedora.DsInfo{}, []byte("hello"))
	tf.Set("test:123", "content", fedora.DsInfo{}, []byte("goodbye"))
	tf.Set("test:abc", "content", fedora.DsInfo{}, []byte("a longer string"))
	tf.Set("another:xyz", "content", fedora.DsInfo{}, []byte("hola"))
	tf.Set("test:badsize", "content", fedora.DsInfo{Size: "0"}, []byte("hola"))
	tf.Set("test:redirect",
		"content",
		fedora.DsInfo{Location: "http://example.com/another/file",
			LocationType: "URL",
			MIMEType:     "audio/mpeg"},
		[]byte("audio stream")) // for DLTP-568
	h := &DownloadHandler{
		Fedora:    tf,
		Ds:        "content",
		Versioned: true,
		Prefix:    "test:",
	}
	return httptest.NewServer(h)
}
