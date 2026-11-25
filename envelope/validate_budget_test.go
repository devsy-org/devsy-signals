package envelope_test

import (
	"bytes"
	"errors"
	"slices"
	"strconv"
	"testing"

	"github.com/devsy-org/devsy-signals/envelope"
)

func TestValidateDataTraversalBudget(t *testing.T) {
	t.Parallel()
	shared := map[string]any{"mixed": []any{[2]any{"a", "b"}}}
	for name, input := range map[string]any{
		"22 array levels": doubledArray(22, 0),
		"40 array levels": doubledArray(40, 0),
		"mixed doubling":  doubledArray(22, shared),
		"leaves":          make([]int, 64<<10),
		"memo hits":       slices.Repeat([]any{shared}, 64<<10),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			e := resultRecord(t)
			e.Data["v"] = input
			checkVerdict(t, "Validate", e.Validate(), envelope.ErrTooLarge)
		})
	}
}

func TestValidateDataBudgetKeepsLargeAndSharedRecords(t *testing.T) {
	t.Parallel()
	e := resultRecord(t)
	shared := []any{[2]int{1, 2}}
	e.Data["v"] = [2]any{make([]int, 30000), slices.Repeat([]any{shared}, 16)}
	checkVerdict(t, "Validate", e.Validate(), nil)
	wire := mustMarshalJSON(t, e)
	if len(wire) < 60000 || len(wire) >= 64<<10 {
		t.Fatalf("expected a near-limit record, got %d bytes", len(wire))
	}
	back, err := envelope.DecodeStrict(wire)
	if err != nil {
		t.Fatal(err)
	}
	checkVerdict(t, "Validate decoded record", back.Validate(), nil)
	if !bytes.Equal(wire, mustMarshalJSON(t, back)) {
		t.Error("round trip changed the record")
	}
}

func BenchmarkValidateArrayDoubling(b *testing.B) {
	for _, levels := range []int{22, 24, 40} {
		b.Run(strconv.Itoa(levels), func(b *testing.B) {
			e := resultRecord(b)
			e.Data["v"] = doubledArray(levels, "leaf")
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				if err := e.Validate(); !errors.Is(err, envelope.ErrTooLarge) {
					b.Fatalf("Validate: got %v, want ErrTooLarge", err)
				}
			}
		})
	}
}

func doubledArray(levels int, value any) any {
	for range levels {
		value = [2]any{value, value}
	}
	return value
}
