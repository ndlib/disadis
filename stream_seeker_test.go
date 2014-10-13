package main

import (
	"strings"
	"testing"
)

func TestStreamSeeker(t *testing.T) {
	r := strings.NewReader("abcdefghijklmnopqrstuvwxyz")
	var p = make([]byte, 10)
	ss := NewStreamSeeker(r, 26)
	off, err := ss.Seek(5, 0)
	if off != 5 || err != nil {
		t.Errorf("Bad offset (%v) (%v)", off, err)
	}
	n, err := ss.Read(p)
	if n != 10 || err != nil {
		t.Errorf("Bad read (%v) (%v)", n, err)
	}
	off, err = ss.Seek(0, 2)
	if off != 26 || err != nil {
		t.Errorf("Bad offset (%v) (%v)", off, err)
	}
	off, err = ss.Seek(-1, 1)
	if off != 25 || err != nil {
		t.Errorf("Bad offset (%v) (%v)", off, err)
	}
	n, err = ss.Read(p)
	if n != 1 || err != nil {
		t.Errorf("Bad read (%v) (%v)", n, err)
	}
}
