//go:build !race

package envelope_test

import (
	"testing"
	"time"

	"github.com/devsy-org/devsy-signals/envelope"
)

func TestValidateArrayDoublingIsBounded(t *testing.T) {
	t.Parallel()
	e := resultRecord(t)
	e.Data["v"] = doubledArray(24, "leaf")
	start := time.Now()
	err := e.Validate()
	if elapsed := time.Since(start); elapsed > 20*time.Millisecond {
		t.Fatalf("24 array levels took %s, want at most 20 ms", elapsed)
	}
	checkVerdict(t, "Validate", err, envelope.ErrTooLarge)
}
