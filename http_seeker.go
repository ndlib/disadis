package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/ndlib/disadis/fedora"
)

// A HTTPSeeker is a ReadSeeker with the ability to seek over content
// available at an HTTP endpoint. It uses range requests to pull data
// as needed into memory. It is optimized for reading of contiguous bytes.
// A token can be supplied for authentication with Bendo.
// If a content length is not supplied, a HEAD request is performed at creation
// to get it.
type HTTPSeeker struct {
	buffer   *bytes.Buffer // data waiting to be read
	pos      int64         // our logical position
	size     int64         // total length of the stream
	Source   string        // data source
	Token    string        // for bendo
	PageSize int           // leave 0 for default
}

// NewHTTPSeeker pages data as needed from the given URL.
// If the end point does not support range requests we will default to a streamseeker.
func NewHTTPSeeker(source string, length int64, token string) *HTTPSeeker {
	result := &HTTPSeeker{
		Source: source,
		Token:  token,
		size:   length,
		buffer: &bytes.Buffer{},
	}
	if length <= 0 {
		result.askcontentlength()
	}
	return result
}

func (h *HTTPSeeker) Close() error { return nil }

// Seek implements the io.Seek() interface
func (h *HTTPSeeker) Seek(offset int64, whence int) (int64, error) {
	var abs int64
	switch whence {
	case 0:
		abs = offset
	case 1:
		abs = h.pos + offset
	case 2:
		abs = h.size + offset
	default:
		return 0, ErrWhence
	}
	if abs > h.size {
		abs = h.size
	}
	// adjust the buffer. This could be more efficient for small forward seeks.
	h.buffer.Reset()
	h.pos = abs
	return abs, nil
}

func (h *HTTPSeeker) Read(p []byte) (n int, err error) {
	// Do we need to fill the buffer?
	if h.buffer.Len() <= 0 {
		bufferSize := 1 << 26 // 64 MiB. arbitrary.
		if h.PageSize > 0 {
			bufferSize = h.PageSize
		}
		startAddr := h.pos
		endAddr := startAddr + int64(bufferSize)
		if endAddr > h.size {
			endAddr = h.size
		}
		if endAddr <= startAddr {
			return 0, io.EOF
		}
		err = h.fillbuffer(startAddr, endAddr)
		if err != nil {
			return 0, err
		}
	}
	n, err = h.buffer.Read(p)
	h.pos += int64(n)
	return
}

func (h *HTTPSeeker) fillbuffer(start int64, end int64) error {
	log.Println("fillbuffer", start, end)
	req, err := http.NewRequest("GET", h.Source, nil)
	if err != nil {
		return err
	}
	req.Header.Add("X-Api-Key", h.Token)
	req.Header.Add("Range", fmt.Sprintf("bytes=%d-%d", start, end-1))
	r, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer r.Body.Close()
	switch r.StatusCode {
	case 404:
		return fedora.ErrNotFound
	case 401:
		return fedora.ErrNotAuthorized
	default:
		return fmt.Errorf("Received status %d from bendo", r.StatusCode)
	case 200, 206:
		// everything is ok
	}
	_, err = io.Copy(h.buffer, r.Body)
	return err
}

func (h *HTTPSeeker) askcontentlength() {
	var err error
	defer func() {
		if err != nil {
			log.Println(h.Source, err)
		}
	}()
	req, err := http.NewRequest("HEAD", h.Source, nil)
	if err != nil {
		return
	}
	req.Header.Add("X-Api-Key", h.Token)
	r, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	defer r.Body.Close()
	if r.StatusCode != 200 {
		log.Println(h.Source, "Received status", r.StatusCode)
	}
	lenstr := r.Header.Get("Content-Length")
	h.size, err = strconv.ParseInt(lenstr, 10, 64)
}
