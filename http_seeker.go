package main

import "net/url"

// A HTTPSeeker is a ReadSeeker with the ability to seek over content
// available at an HTTP endpoint. It uses range requests to pull data
// as needed into memory.
type HTTPSeeker struct {
	pos  int64 // our logical position
	i    int64 // the logical starting position of the buffer
	size int64 // total length of the stream
}

// NewHTTPSeeker pages data as needed from the given URL. It will try to pull the
// content in chunks of pagesize size. If pagesize is 0 we use the default size.
// If the endpoint does not support range requests we will default to a streamseeker.
func NewHTTPSeeker(target url.URL) *HTTPSeeker {
	return &HTTPSeeker{}
}

// Seek implements the io.Seek() interface
func (hs *HTTPSeeker) Seek(offset int64, whence int) (int64, error) {
	var abs int64
	switch whence {
	case 0:
		abs = offset
	case 1:
		abs = hs.pos + offset
	case 2:
		abs = hs.size + offset
	default:
		return 0, ErrWhence
	}
	if abs < hs.i {
		return 0, ErrInvalidPos
	}
	if abs > hs.size {
		abs = hs.size
	}
	hs.pos = abs
	return abs, nil
}

func (hs *HTTPSeeker) Read(p []byte) (n int, err error) {
	// do we need to read a bit to catch up to the logical position?
	for ss.i < ss.pos {
		// reuse the buffer we were given to do this
		var pp = p
		if ss.i+int64(len(pp)) > ss.pos {
			pp = p[0 : ss.pos-ss.i]
		}
		n, err := ss.s.Read(pp)
		if err != nil {
			return 0, err
		}
		ss.i += int64(n)
	}
	// read into p
	n, err = ss.s.Read(p)
	if err == nil {
		ss.i += int64(n)
		ss.pos += int64(n)
	}
	return n, err
}
