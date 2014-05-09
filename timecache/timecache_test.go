package timecache

import (
	"testing"
	"time"
)

func TestPrune(t *testing.T) {
	var (
		tc = timecache{
			data: make([]entry, 50),
		}
		table = []struct{ head, tail, newTail int }{
			{25, 10, 13},
			{11, 10, 11},
			{5, 48, 1},
		}
		delta = 5 * time.Second
		now   = time.Now()
	)

	for _, test := range table {
		var (
			v = now.Add(-2 * delta)
			i = test.tail
		)
		tc.head = test.head
		tc.tail = test.tail
		for i != test.head {
			tc.data[i] = entry{t: v}
			v = v.Add(delta)
			i++
			if i >= len(tc.data) {
				i = 0
			}
		}
		tc.prune()
		if tc.tail != test.newTail {
			t.Fatalf("Prune moved tail to %d instead of %d", tc.tail, test.newTail)
		}
	}
}

type mycount struct {
	n int
}

func TestAdd(t *testing.T) {
	var tc = New(5, time.Second)

	tc.Add("a", mycount{n: 1})
	tc.Add("b", mycount{n: 2})
	tc.Add("c", mycount{n: 3})

	v, err := tc.Get("b")
	if err != nil {
		t.Fatalf("Got error %v", err)
	}
	n, ok := v.(mycount)
	if !ok {
		t.Fatalf("Did not receive a mycount")
	}
	if n.n != 2 {
		t.Fatalf("Expected 2, got %v", n)
	}
}
