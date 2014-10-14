package main

import (
	"errors"
	"io"
)

// Allow ability to seek in a stream
// However, can only seek to positions that not been read yet.
type StreamSeeker struct {
	s    io.Reader
	pos  int64 // our logical position
	i    int64 // number of bytes read so far
	size int64 // total length of the stream
}

var (
	seekerError = errors.New("StreamSeeker.Seek: cannot seek before read position")
	whenceError = errors.New("StreamSeeker.Seek: invalid whence")
)

// Returns a StreamSeeker wrapping s, and allowing a maximum size of size.
// size does not need to set, but it does to allow for seeks relative to the end.
func NewStreamSeeker(s io.Reader, size int64) *StreamSeeker {
	return &StreamSeeker{
		s:    s,
		size: size,
	}
}

func (ss *StreamSeeker) Seek(offset int64, whence int) (int64, error) {
	var abs int64
	switch whence {
	case 0:
		abs = offset
	case 1:
		abs = ss.pos + offset
	case 2:
		abs = ss.size + offset
	default:
		return 0, whenceError
	}
	if abs < ss.i {
		return 0, seekerError
	}
	if abs > ss.size {
		return 0, seekerError
	}
	ss.pos = abs
	return abs, nil
}

func (ss *StreamSeeker) Read(p []byte) (n int, err error) {
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
