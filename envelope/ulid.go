package envelope

import (
	"crypto/rand"
	"io"
	"sync"
	"time"
)

const crockford = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

//nolint:gochecknoglobals // one process-wide generator is what keeps IDs monotonic across callers
var defaultGenerator = &generator{now: time.Now, rand: rand.Reader}

// NewID returns a new ULID. IDs from one process are strictly increasing, even
// within one millisecond or when the clock steps back.
func NewID() string {
	return defaultGenerator.next()
}

type generator struct {
	mu      sync.Mutex
	now     func() time.Time
	rand    io.Reader
	started bool
	lastMS  uint64
	hi      uint16
	lo      uint64
}

func (g *generator) next() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	ms := uint64(g.now().UnixMilli())
	switch {
	case !g.started || ms > g.lastMS:
		g.started = true
		g.lastMS = max(ms, g.lastMS)
		g.refill()
	case !g.increment():
		// Entropy ran out inside one millisecond. Moving the timestamp on by
		// 1 ms keeps IDs monotonic; the 2^80 space makes this practically
		// unreachable.
		g.lastMS++
		g.refill()
	}
	return encodeULID(g.lastMS, g.hi, g.lo)
}

func (g *generator) refill() {
	var b [10]byte
	if _, err := io.ReadFull(g.rand, b[:]); err != nil {
		panic("envelope: random source failed: " + err.Error())
	}
	g.hi = uint16(b[0])<<8 | uint16(b[1])
	g.lo = 0
	for _, x := range b[2:] {
		g.lo = g.lo<<8 | uint64(x)
	}
}

// increment adds one to the 80-bit entropy and reports false on overflow.
func (g *generator) increment() bool {
	g.lo++
	if g.lo != 0 {
		return true
	}
	g.hi++
	return g.hi != 0
}

func encodeULID(ms uint64, hi uint16, lo uint64) string {
	h := (ms&(1<<48-1))<<16 | uint64(hi)
	var out [26]byte
	for i := len(out) - 1; i >= 0; i-- {
		out[i] = crockford[lo&31]
		lo = lo>>5 | h<<59
		h >>= 5
	}
	return string(out[:])
}
