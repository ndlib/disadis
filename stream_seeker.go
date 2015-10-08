package main

import (
	"errors"
	"io"
)

// A StreamSeeker is a Reader with the ability to seek in a stream.
// However, it can only seek to positions that not been read yet.
type StreamSeeker struct {
	s    io.Reader // the Reader we are wrapping
	pos  int64     // our logical position
	i    int64     // number of bytes read so far, can go behind this point
	size int64     // total length of the stream
}

// Stream seeker errors.
var (
	ErrInvalidPos = errors.New("StreamSeeker.Seek: cannot seek before read position")
	ErrWhence     = errors.New("StreamSeeker.Seek: invalid whence")
)

// NewStreamSeeker wraps the given reader. size is the maximum size of the
// stream s refers to. Seeks past the maximum size are not allowed. (But reads
// past there are). Size does not need to valid, but providing it allows for
// seeks relative to the end. In particular, the standard http library uses
// seeking as the way to determine the size of the stream.
func NewStreamSeeker(s io.Reader, size int64) *StreamSeeker {
	return &StreamSeeker{
		s:    s,
		size: size,
	}
}

// Seek implements the io.Seek() interface on a StreamSeeker.
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
		return 0, ErrWhence
	}
	if abs < ss.i {
		return 0, ErrInvalidPos
	}
	if abs > ss.size {
		return 0, ErrInvalidPos
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
