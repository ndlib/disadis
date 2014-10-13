package main

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dbrower/disadis/fedora"
)

func TestDownload(t *testing.T) {
	tFedora := fedora.NewTestFedora()
	tFedora.Set("test:0123", "content", []byte("hello"))
	tFedora.Set("test:123", "content", []byte("goodbye"))
	h := &DownloadHandler{
		Fedora:    tFedora,
		Ds:        "content",
		Versioned: true,
		Prefix:    "test:",
	}
	ts := httptest.NewServer(h)
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
		// identifiers are assumed to not have more than 64 characters
		{"GET", "/1234567890123456789012345678901234567890123456789012345", 404, ""},
	}
	for _, s := range sequence {
		checkRoute(t, s.verb, ts.URL+s.route, s.status, s.expected)
	}
}

func checkRoute(t *testing.T, verb, route string, status int, expected string) {
	req, err := http.NewRequest(verb, route, nil)
	if err != nil {
		t.Fatal("Problem creating request", err)
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
	if expected != "" {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(route, err)
		}
		if string(body) != expected {
			t.Errorf("%s: Expected body %s, got %v",
				route,
				expected,
				body)
		}
	}
	resp.Body.Close()
}
