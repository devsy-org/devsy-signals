package envelope_test

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"testing"
	"time"
	"unsafe"

	"github.com/devsy-org/devsy-signals/envelope"
	"github.com/devsy-org/devsy-signals/schema"
)

func TestValidateDataRejectsUnsupportedMapKeys(t *testing.T) {
	t.Parallel()
	key := "key\x00"
	clean := "clean"
	const payload = "payload"
	for name, value := range map[string]any{
		"interface NUL": map[any]string{key: "x"},
		"pointer NUL":   map[*string]string{&key: "x"},
		"clean pointer": map[*string]string{&clean: "x"},
		"nil interface": map[any]string{nil: "x"},
		"nil pointer":   map[*string]string{nil: "x"},
		"float":         map[float64]string{1: "x"},
		"empty map":     map[bool]string{},
		"nil map":       map[bool]string(nil),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			e := resultRecord(t)
			e.Data[payload] = value
			if err := e.Validate(); err == nil {
				t.Error("Validate accepted a nonportable map key type")
			}
			if strings.HasSuffix(name, "NUL") {
				checkVerdict(t, "Validate resolved NUL key", e.Validate(), envelope.ErrText)
				// Go 1.23 rejects the key type at marshal time; newer encoders can emit it.
				if wire, err := e.MarshalJSON(); err == nil {
					_, err := envelope.DecodeStrict(wire)
					checkVerdict(t, "DecodeStrict", err, envelope.ErrText)
				}
			}
		})
	}
}

func TestValidateDataMapKeyWireForms(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		value any
		key   string
	}{
		{map[plainWireString]bool{"literal": true}, "literal"},
		{map[int16]bool{-12: true}, "-12"},
		{map[uint64]bool{math.MaxUint64: true}, "18446744073709551615"},
		{map[uintptr]bool{4: true}, "4"},
	} {
		t.Run(tc.key, func(t *testing.T) {
			t.Parallel()
			e := resultRecord(t)
			e.Data["payload"] = tc.value
			checkDataRoundTripVerdict(t, e, nil)
			checkWireFragments(t, mustMarshalJSON(t, e), fmt.Sprintf(`"payload":{%q:true}`, tc.key))
		})
	}
}

func TestValidateDataRejectsUnencodableScalars(t *testing.T) {
	t.Parallel()
	var ptr unsafe.Pointer
	for name, value := range map[string]any{
		"NaN":            math.NaN(),
		"positive Inf":   math.Inf(1),
		"negative Inf":   float32(math.Inf(-1)),
		"bad number":     json.Number("bad"),
		"complex64":      complex64(1i),
		"complex128":     complex128(1i),
		"chan":           (chan int)(nil),
		"func":           (func())(nil),
		"unsafe pointer": ptr,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			e := resultRecord(t)
			e.Data["payload"] = value
			if _, err := e.MarshalJSON(); err == nil {
				t.Error("MarshalJSON accepted an unencodable value")
			}
			if err := e.Validate(); err == nil {
				t.Error("data validation accepted an unencodable value")
			}
			delete(e.Data, "payload")
			e.Attrs = map[string]any{"payload": value}
			if err := e.Validate(); err == nil {
				t.Error("attrs validation accepted an unencodable value")
			}
		})
	}
}

func TestValidateDataKeepsJSONScalars(t *testing.T) {
	t.Parallel()
	e := resultRecord(t)
	e.Data["payload"] = []any{1.5, -2, uint(3), true, nil, json.Number("1e400"), []byte("ab")}
	checkDataRoundTripVerdict(t, e, nil)
	checkWireFragments(t, mustMarshalJSON(t, e), `"payload":[1.5,-2,3,true,null,1e400,"YWI="]`)
}

func TestValidateSharedData(t *testing.T) {
	t.Parallel()
	e := resultRecord(t)
	e.Data["payload"] = sharedData(18)
	start := time.Now()
	checkVerdict(t, "Validate shared data", e.Validate(), envelope.ErrTooLarge)
	if elapsed := time.Since(start); elapsed > 5*time.Millisecond {
		t.Errorf("validation took %v, want under 5ms", elapsed)
	}
	e.Data["payload"] = sharedData(12)
	checkDataRoundTripVerdict(t, e, nil)
}

func TestValidateSharedDataChecksEachDepth(t *testing.T) {
	t.Parallel()
	e := resultRecord(t)
	shared := []any{[]any{}}
	var deep any = shared
	for range schema.MaxRecordDepth - 4 {
		deep = []any{deep}
	}
	e.Data["payload"] = []any{shared, deep}
	checkDataRoundTripVerdict(t, e, envelope.ErrDepth)
}

func sharedData(levels int) any {
	var node any = "leaf"
	for range levels {
		node = []any{node, node}
	}
	return node
}

func BenchmarkValidateSharedData(b *testing.B) {
	for _, n := range []int{8, 12, 15} {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			e := resultRecord(b)
			e.Data["payload"] = sharedData(n)
			benchmarkData(b, e)
		})
	}
}

func BenchmarkValidateWideData(b *testing.B) {
	const recordLimit = 64 << 10
	list := strings.Repeat(`"x",`, 9999) + `"x"`
	prefix := `,"data":{"list":[` + list + `],"pad":"`
	wire := resultWith(prefix + `"}`)
	wire = resultWith(prefix + strings.Repeat("x", recordLimit-len(wire)) + `"}`)
	e, err := envelope.DecodeStrict([]byte(wire))
	if err != nil {
		b.Fatal(err)
	}
	b.SetBytes(int64(len(wire)))
	benchmarkData(b, e)
}

func benchmarkData(b *testing.B, e envelope.Envelope) {
	b.Helper()
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if err := e.Validate(); err != nil {
			b.Fatal(err)
		}
	}
}
