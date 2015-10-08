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

func verifyEntry(tc Cache, key string, exp mycount, exists bool, t *testing.T) {
	v, err := tc.Get(key)
	if err != nil && exists {
		t.Fatalf("Could not find entry %s %v", key, exp)
	}
	if !exists && err == nil {
		t.Fatalf("Found unexpected entry %s %v", key, exp)
	}
	if !exists {
		return
	}
	m, ok := v.(mycount)
	if !ok {
		t.Fatalf("Did not receive a mycount for %s %v", key, exp)
	}
	if m.n != exp.n {
		t.Fatalf("key %s: expected %v, got %v", key, exp, m)
	}
}

func TestAdd(t *testing.T) {
	var tc = New(5, time.Second)

	tc.Add("a", mycount{n: 1})
	tc.Add("b", mycount{n: 2})
	tc.Add("c", mycount{n: 3})
	tc.Add("b", mycount{n: 4})

	verifyEntry(tc, "a", mycount{n: 1}, true, t)
	verifyEntry(tc, "b", mycount{n: 2}, true, t)
	verifyEntry(tc, "c", mycount{n: 3}, true, t)

	tc.Add("d", mycount{n: 5})
	tc.Add("e", mycount{n: 6})
	tc.Add("f", mycount{n: 7})
	tc.Add("g", mycount{n: 8})
	tc.Add("h", mycount{n: 9})

	verifyEntry(tc, "a", mycount{n: 1}, false, t)
	verifyEntry(tc, "b", mycount{n: 2}, false, t)
	verifyEntry(tc, "c", mycount{n: 3}, false, t)
	verifyEntry(tc, "d", mycount{n: 5}, true, t)
	verifyEntry(tc, "e", mycount{n: 6}, true, t)
	verifyEntry(tc, "f", mycount{n: 7}, true, t)
	verifyEntry(tc, "g", mycount{n: 8}, true, t)
	verifyEntry(tc, "h", mycount{n: 9}, true, t)
}

func TestTimeout(t *testing.T) {
	var tc = New(5, time.Millisecond)

	tc.Add("a", mycount{n: 1})
	time.Sleep(5 * time.Millisecond)
	tc.Add("b", mycount{n: 2})
	time.Sleep(5 * time.Millisecond)
	tc.Add("c", mycount{n: 3})

	verifyEntry(tc, "a", mycount{n: 1}, false, t)
	verifyEntry(tc, "b", mycount{n: 4}, false, t)
	verifyEntry(tc, "c", mycount{n: 3}, true, t)
}
