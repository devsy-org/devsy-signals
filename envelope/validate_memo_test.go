package envelope_test

import (
	"encoding/json"
	"errors"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/devsy-org/devsy-signals/envelope"
)

const (
	chainHops = 128
	chainRefs = 16000
	memoField = "memo"
)

func TestValidateMemoChargesExpandedValues(t *testing.T) {
	t.Parallel()
	for _, levels := range []int{17, 20, 40} {
		t.Run(strconv.Itoa(levels), func(t *testing.T) {
			t.Parallel()
			e := resultRecord(t)
			e.Data[memoField] = doubledSlice(levels)
			checkVerdict(t, "Validate", e.Validate(), envelope.ErrTooLarge)
		})
	}
}

func TestValidatePointerChain(t *testing.T) {
	t.Parallel()
	e := pointerChainRecord(t)
	checkVerdict(t, "Validate", e.Validate(), nil)
	_, err := envelope.DecodeStrict(mustMarshalJSON(t, e))
	checkVerdict(t, "DecodeStrict", err, nil)
}

func TestValidateExpandedBudgetBoundary(t *testing.T) {
	t.Parallel()
	e := resultRecord(t)
	e.Data[""] = doubledSlice(15) // 65,535 one-byte charges plus the data object.
	checkVerdict(t, "at budget", e.Validate(), nil)
	e.Data["extra"] = 0
	checkVerdict(t, "over budget", e.Validate(), envelope.ErrTooLarge)
}

func TestValidateMemoKeepsNearLimitRecord(t *testing.T) {
	t.Parallel()
	e := resultRecord(t)
	e.Data[memoField] = slices.Repeat([]any{doubledSlice(7)}, 120)
	e.Data["memo-padding"] = ""
	e.Data["memo-padding"] = strings.Repeat("p", 64893-len(mustMarshalJSON(t, e)))
	checkVerdict(t, "Validate", e.Validate(), nil)
	wire := mustMarshalJSON(t, e)
	if len(wire) != 64893 {
		t.Fatalf("expected a 64,893-byte record, got %d bytes", len(wire))
	}
	_, err := envelope.DecodeStrict(wire)
	checkVerdict(t, "DecodeStrict", err, nil)
}

func TestValidateIndirectPointerCycles(t *testing.T) {
	t.Parallel()
	e := resultRecord(t)
	var boxed any
	boxed = &boxed
	e.Data[memoField] = pointerChain(boxed, 3)
	checkVerdict(t, "interface cycle", e.Validate(), envelope.ErrDepth)

	type recursivePointer *recursivePointer
	var ptr recursivePointer
	ptr = &ptr
	e.Data[memoField] = ptr
	checkVerdict(t, "pointer cycle", e.Validate(), envelope.ErrDepth)
}

func TestValidatePointerMemoSeparatesTypes(t *testing.T) {
	t.Parallel()
	e := resultRecord(t)
	value := plainWireString("note")
	plain := pointerChain(&value, 2)
	e.Data[memoField] = []any{plain, pointerChain(plain, 1), (*json.Number)(&value)}
	if err := e.Validate(); err == nil {
		t.Fatal("pointer memo treated malformed json.Number as an ordinary string")
	}
	e.Data[memoField] = []any{
		plain,
		pointerChain(plain, 1),
		pointerChain((*jsonMarshalerString)(&value), 2),
	}
	checkVerdict(t, "marshaler behind cached chain and alias", e.Validate(), envelope.ErrText)
}

func BenchmarkValidatePointerChain(b *testing.B) {
	benchmarkData(b, pointerChainRecord(b))
}

func BenchmarkValidateMemoDoubling(b *testing.B) {
	for _, levels := range []int{14, 17, 20, 40} {
		b.Run(strconv.Itoa(levels), func(b *testing.B) {
			e := resultRecord(b)
			e.Data[memoField] = doubledSlice(levels)
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				if err := e.Validate(); err != nil && !errors.Is(err, envelope.ErrTooLarge) {
					b.Fatal(err)
				}
			}
		})
	}
}

func pointerChainRecord(tb testing.TB) envelope.Envelope {
	tb.Helper()
	e := resultRecord(tb)
	e.Data[memoField] = slices.Repeat([]any{pointerChain(0, chainHops)}, chainRefs)
	return e
}

func pointerChain(value any, hops int) any {
	for range hops {
		next := value
		value = &next
	}
	return value
}

func doubledSlice(levels int) any {
	var value any = 0
	for range levels {
		value = []any{value, value}
	}
	return value
}
