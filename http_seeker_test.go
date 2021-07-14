package main

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

func TestHTTPSeeker(t *testing.T) {
	var table = []struct {
		path   string
		length int
		data   []byte
	}{
		{path: "/10", length: 10, data: []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}},
		{path: "/30", length: 30, data: []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0xa, 0xb, 0xc, 0xd, 0xe, 0xf, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0xa, 0xb, 0xc, 0xd}},
	}

	for _, tab := range table {
		s := NewHTTPSeeker(RangeServer.URL+tab.path, 0, "")
		s.PageSize = 13
		data, err := io.ReadAll(s)
		t.Log(len(data))
		if len(data) != tab.length {
			t.Errorf("%s Got %d. Expected %d", tab.path, len(data), tab.length)
		}
		if !bytes.Equal(data, tab.data) {
			t.Errorf("%s Got %v. Expected %v", tab.path, data, tab.data)
		}
		if err != nil {
			t.Error("Received error", err)
		}
		t.Log(s)
	}
}

// An RangeTarget is a simple handler that supports range requests.
// All files have zero byte contents, and length is given in the path.
// e.g. /12345 is a file consisting of 12,345 zero octets.
type RangeTarget struct{}

func (t *RangeTarget) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Println(r.Method, r.URL.Path)
	length, _ := strconv.Atoi(r.URL.Path[1:])

	http.ServeContent(w, r, "", time.Time{}, &zeroreader{length: int64(length)})
}

type zeroreader struct {
	pos    int64
	length int64
}

func (z *zeroreader) Read(p []byte) (int, error) {
	var data = []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f,
		0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f,
		0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f,
		0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f,
	}

	// figure out the most we can copy out of data
	n := len(data)
	pos := int(z.pos) % n
	n = n - pos
	// adjust so we don't go longer than our logical length
	if n > int(z.length-z.pos) {
		n = int(z.length - z.pos)
	}
	if n <= 0 {
		return 0, io.EOF
	}
	// don't copy any more than the receiving buffer
	if len(p) < n {
		n = len(p)
	}

	z.pos += int64(n)
	copy(p, data[pos:pos+n])
	return n, nil
}

func (z *zeroreader) Seek(offset int64, whence int) (int64, error) {
	var abs int64
	switch whence {
	case 0:
		abs = offset
	case 1:
		abs = z.pos + offset
	case 2:
		abs = z.length + offset
	}
	if abs > z.length {
		abs = z.length
	}
	z.pos = abs
	return abs, nil
}

var RangeServer *httptest.Server

func init() {
	RangeServer = httptest.NewServer(&RangeTarget{})
}
