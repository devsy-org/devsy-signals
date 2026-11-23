package envelope

import (
	"bytes"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestEncodeULIDVectors(t *testing.T) {
	t.Parallel()
	cases := []struct {
		ms   uint64
		hi   uint16
		lo   uint64
		want string
	}{
		{0, 0, 0, "00000000000000000000000000"},
		{1<<48 - 1, 0xffff, 1<<64 - 1, "7ZZZZZZZZZZZZZZZZZZZZZZZZZ"},
		{1, 0, 0, "00000000010000000000000000"},
		{0, 0, 31, "0000000000000000000000000Z"},
		{0, 0, 32, "00000000000000000000000010"},
	}
	for _, c := range cases {
		if got := encodeULID(c.ms, c.hi, c.lo); got != c.want {
			t.Errorf("encodeULID(%d,%d,%d) = %s, want %s", c.ms, c.hi, c.lo, got, c.want)
		}
	}
}

func TestEncodeULIDTimestamp(t *testing.T) {
	t.Parallel()
	// Timestamp of the example in the ULID spec.
	if got := encodeULID(1469918176385, 0, 0)[:10]; got != "01ARYZ6S41" {
		t.Fatalf("got %s", got)
	}
}

func TestNewIDFormatAndOrder(t *testing.T) {
	t.Parallel()
	ulidRE := regexp.MustCompile(`^[0-7][0-9A-HJKMNP-TV-Z]{25}$`)
	prev := ""
	for range 20000 {
		id := NewID()
		if !ulidRE.MatchString(id) {
			t.Fatalf("bad ULID %q", id)
		}
		if id <= prev {
			t.Fatalf("%s is not after %s", id, prev)
		}
		prev = id
	}
}

func TestGeneratorMonotonicWithFrozenAndBackwardClock(t *testing.T) {
	t.Parallel()
	now := time.UnixMilli(1_700_000_000_000)
	g := &generator{now: func() time.Time { return now }, rand: bytes.NewReader(make([]byte, 1000))}
	first := g.next()
	second := g.next()
	now = now.Add(-time.Hour)
	third := g.next()
	if first >= second || second >= third {
		t.Fatalf("not increasing: %s %s %s", first, second, third)
	}
	if first[:10] != second[:10] || first[:10] != third[:10] {
		t.Fatalf("timestamp prefix changed: %s %s %s", first, second, third)
	}
	now = now.Add(time.Hour + time.Millisecond)
	if fourth := g.next(); fourth <= third || fourth[:10] == third[:10] {
		t.Fatalf("clock advance not reflected: %s after %s", fourth, third)
	}
}

func TestGeneratorEntropyOverflowAdvancesTime(t *testing.T) {
	t.Parallel()
	g := &generator{
		now:     func() time.Time { return time.UnixMilli(5) },
		rand:    bytes.NewReader(bytes.Repeat([]byte{0xff}, 100)),
		started: true,
		lastMS:  5, hi: 0xffff, lo: 1<<64 - 1,
	}
	id := g.next()
	if g.lastMS != 6 || id <= encodeULID(5, 0xffff, 1<<64-1) {
		t.Fatalf("lastMS=%d id=%s", g.lastMS, id)
	}
}

func TestGeneratorIncrementCarriesIntoHigh(t *testing.T) {
	t.Parallel()
	g := &generator{
		now:     func() time.Time { return time.UnixMilli(5) },
		started: true,
		lastMS:  5, hi: 0x7fff, lo: 1<<64 - 1,
	}
	id := g.next()
	if g.lastMS != 5 || id != encodeULID(5, 0x8000, 0) {
		t.Fatalf("lastMS=%d id=%s", g.lastMS, id)
	}
}

func TestGeneratorPanicsWithoutEntropy(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected a panic")
		}
	}()
	g := &generator{now: time.Now, rand: strings.NewReader("")}
	g.next()
}

func TestNewIDConcurrentUnique(t *testing.T) {
	t.Parallel()
	var mu sync.Mutex
	seen := map[string]bool{}
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 1000 {
				id := NewID()
				mu.Lock()
				if seen[id] {
					t.Errorf("duplicate %s", id)
				}
				seen[id] = true
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
}

func TestGeneratorUsesAll80Bits(t *testing.T) {
	t.Parallel()
	g := &generator{
		now:  func() time.Time { return time.UnixMilli(1_700_000_000_000) },
		rand: bytes.NewReader(bytes.Repeat([]byte{0xff}, 10)),
	}
	if id := g.next(); id[10:] != "ZZZZZZZZZZZZZZZZ" {
		t.Fatalf("entropy lost bits: %s", id)
	}
}

func TestGeneratorSeedsEntropyAtMillisecondZero(t *testing.T) {
	t.Parallel()
	g := &generator{
		now:  func() time.Time { return time.UnixMilli(0) },
		rand: bytes.NewReader(bytes.Repeat([]byte{0x80}, 10)),
	}
	if id := g.next(); id[10:] == "0000000000000001" {
		t.Fatalf("not seeded from entropy: %s", id)
	}
	if id := g.next(); id[:10] != "0000000000" {
		t.Fatalf("timestamp moved: %s", id)
	}
}
