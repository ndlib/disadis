package timecache

import (
	"errors"
	"sync"
	"time"
)

// Cache provides a simple interface for caches having string keys.
type Cache interface {
	Get(key string) (interface{}, error)
	Add(key string, value interface{}) error
}

// Contains a timecache structure
type timecache struct {
	sync.RWMutex               // Protects everything below
	expires      time.Duration // length of time until items are removed

	// `data` is a circular buffer. head points to first empty
	// space to store new entries, tail points to oldest non-empty square.
	// head == tail iff the cache is empty. This means we can only
	// utilize len(data)-1 spaces.
	head, tail int
	data       []entry
	stop       chan<- struct{} // stop the background sweeper
}

type entry struct {
	key string
	t   time.Time // the time at which this item expires
	val interface{}
}

var (
	// ErrNotFound indicates a key was not found in the cache
	ErrNotFound = errors.New("not found")
)

// Gets the oldest item in the cache with the given key.
// returns the ErrNotFound error if nothing is found.
// Item is guarenteed to not be older than expires.
func (tc *timecache) Get(key string) (interface{}, error) {
	var (
		now = time.Now()
		v   interface{}
		err = ErrNotFound
	)
	tc.RLock()
	for i := tc.tail; i != tc.head; i = tc.advance(i) {
		if key == tc.data[i].key && now.Before(tc.data[i].t) {
			v = tc.data[i].val
			err = nil
			break
		}
	}
	tc.RUnlock()
	return v, err
}

// Add item with key to the cache. Duplicate keys can be added, only the oldest
// unexpired one is returned for any call to Get().
func (tc *timecache) Add(key string, value interface{}) error {
	// We will add the key to the end, even if it already exists!
	now := time.Now()
	tc.Lock()
	var newHead = tc.advance(tc.head)
	tc.data[tc.head].key = key
	tc.data[tc.head].t = now.Add(tc.expires)
	tc.data[tc.head].val = value
	tc.head = newHead
	if newHead == tc.tail {
		// cache full, advance tail
		tc.tail = tc.advance(tc.tail)
	}
	tc.Unlock()
	return nil
}

// New creates a timecache which can store a maximum of size entires
// and entries will be deleted after expires time has elapsed.
func New(size int, expires time.Duration) *timecache {
	c := make(chan struct{})
	// add 1 to the size because the way we use head and tail
	// the maximum capacity of the cache is len(data) - 1
	tc := &timecache{
		expires: expires,
		data:    make([]entry, size+1),
		stop:    c,
	}
	go tc.backgroundCleaner(c)
	return tc
}

func (tc *timecache) advance(i int) int {
	i++
	if i >= len(tc.data) {
		i = 0
	}
	return i
}

func (tc *timecache) backgroundCleaner(stop <-chan struct{}) {
	var nextTime time.Time
Outer:
	for {
		var timeout time.Duration = 5 * time.Minute
		if (!nextTime.IsZero()) && nextTime.Before(time.Now()) {
			timeout = nextTime.Sub(time.Now())
		}
		select {
		case <-stop:
			break Outer
		case <-time.After(timeout):
			nextTime = tc.prune()
		}
	}
}

// prune expired entries from cache.
// returns the expiry time of the oldest entry remaining in cache.
// If the cache is empty, returns the 0 time.
// It is possible a non-zero time may be returned if the cache is empty.
func (tc *timecache) prune() time.Time {
	var oldestTime time.Time
	var now = time.Now()

	// First lock in read only mode to see if anything needs pruning.
	// If so, we relock as a writer and do it for real.
	tc.RLock()
	if tc.head == tc.tail {
		// cache is empty
		tc.RUnlock()
		return oldestTime
	}
	// we only need to peek at the tail
	oldestTime = tc.data[tc.tail].t
	tc.RUnlock()
	if now.Before(oldestTime) {
		return oldestTime
	}
	tc.Lock()
	var i = tc.tail
	for ; i != tc.head; i = tc.advance(i) {
		oldestTime = tc.data[i].t
		if now.Before(oldestTime) {
			break
		}
	}
	tc.tail = i
	tc.Unlock()
	// if we removed all the items from cache, then oldestTime will be
	// in error since it will not be zero. We are okay with that.
	return oldestTime
}
