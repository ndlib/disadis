package main

import (
	"strings"
	"testing"
)

func TestStreamSeeker(t *testing.T) {
	r := strings.NewReader("abcdefghijklmnopqrstuvwxyz")
	var p = make([]byte, 10)
	ss := NewStreamSeeker(r, 26)
	// seek to five from beginning, read 10
	off, err := ss.Seek(5, 0)
	if off != 5 || err != nil {
		t.Errorf("Bad offset (%v) (%v)", off, err)
	}
	n, err := ss.Read(p)
	if n != 10 || err != nil {
		t.Errorf("Bad read (%v) (%v)", n, err)
	}
	// seek to end
	off, err = ss.Seek(0, 2)
	if off != 26 || err != nil {
		t.Errorf("Bad offset (%v) (%v)", off, err)
	}
	// seek back one byte, read
	off, err = ss.Seek(-1, 1)
	if off != 25 || err != nil {
		t.Errorf("Bad offset (%v) (%v)", off, err)
	}
	n, err = ss.Read(p)
	if n != 1 || err != nil {
		t.Errorf("Bad read (%v) (%v)", n, err)
	}
	// seek with invalid whence
	off, err = ss.Seek(0, 3)
	if err != ErrWhence {
		t.Errorf("Expected error for bad whence, got %v", err)
	}
	// seek past end
	off, err = ss.Seek(1, 2)
	if err != ErrInvalidPos {
		t.Errorf("Expected error for seek past end, got %v", err)
	}
	// seek before beginning of stream
	off, err = ss.Seek(-1, 0)
	if err != ErrInvalidPos {
		t.Errorf("Expected error for seek before beginning, got %v", err)
	}
}
