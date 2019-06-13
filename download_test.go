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

		{"GET", "/123", 200, "goodbye"},
		{"HEAD", "/123", 200, ""},

		{"GET", "/0123?datastream_id=content", 200, "hello"},
		{"POST", "/0123", 405, ""},

		{"GET", "/badsize", 200, "hola"},

		// It applies the correct prefix
		{"GET", "/xyz", 404, ""},
		{"HEAD", "/xyz", 404, ""},

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

// Check that redirects use the token, if supplied
func TestRedirectToken(t *testing.T) {
	ts := setupHandler()
	defer ts.Close()

	checkRoute(t, "GET", ts.URL+"/remote", 200, "")

	// make token invalid
	// and see if we get an unquthorized error.
	// this is dirty, but it is just a test.
	ts.Config.Handler.(*DownloadHandler).BendoToken = "abc"
	checkRoute(t, "GET", ts.URL+"/remote", 500, "")

	// remove token config
	// and see if we get the fedora response
	// this is dirty, but it is just a test.
	ts.Config.Handler.(*DownloadHandler).BendoToken = ""
	checkRoute(t, "GET", ts.URL+"/remote", 200, "from fedora")
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

// An AuthTarget is a simple handler that returns 200 if
// a correct token is provided in the X-Api-Key header.
// Otherwise, a 401 is returned.
type AuthTarget struct {
	Tokens []string
}

func (t *AuthTarget) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	goal := r.Header.Get("X-Api-Key")
	// token in list?
	for _, token := range t.Tokens {
		if goal == token {
			return
		}
	}
	w.WriteHeader(http.StatusUnauthorized)
}

var BendoServer *httptest.Server

func init() {
	BendoServer = httptest.NewServer(&AuthTarget{
		Tokens: []string{"12345"},
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
		fedora.DsInfo{
			Location:     BendoServer.URL + "/another/file",
			LocationType: "URL",
			MIMEType:     "audio/mpeg"},
		[]byte("audio stream")) // for DLTP-568
	tf.Set("test:remote",
		"content",
		fedora.DsInfo{
			Location:     BendoServer.URL + "/test",
			LocationType: "URL",
			MIMEType:     "image/png",
		},
		[]byte("from fedora"))
	h := &DownloadHandler{
		Fedora:     tf,
		Ds:         "content",
		Prefix:     "test:",
		BendoToken: "12345",
	}
	return httptest.NewServer(h)
}
