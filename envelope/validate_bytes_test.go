package envelope_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/devsy-org/devsy-signals/envelope"
)

func TestValidateRepeatedLargeString(t *testing.T) {
	t.Parallel()
	e := repeatedLargeString(t)
	start := time.Now()
	err := e.Validate()
	checkVerdict(t, "Validate", err, envelope.ErrTooLarge)
	if elapsed := time.Since(start); elapsed > 250*time.Millisecond {
		t.Errorf("Validate took %v, want under 250 ms", elapsed)
	}
}

func TestValidateDataByteCharges(t *testing.T) {
	t.Parallel()
	const field = "expanded"
	block := strings.Repeat("a", 4096)
	oversize := strings.Repeat("a", 70000)
	within := strings.Repeat("a", 60000)
	for name, data := range map[string]map[string]any{
		"string":              {field: oversize},
		"repeated text":       {field: slices.Repeat([]string{block}, 16000)},
		"memoized text":       {field: slices.Repeat([]any{[]string{block}}, 17)},
		"long object key":     {oversize: true},
		"repeated key":        {field: slices.Repeat([]any{map[string]bool{block: true}}, 17)},
		"bytes":               {field: make([]byte, 70000)},
		"number":              {field: json.Number(strings.Repeat("1", 70000))},
		"oversize before NUL": {field: oversize + "\x00"},
		"oversize key NUL":    {oversize + "\x00": true},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			e := resultRecord(t)
			e.Data = data
			checkVerdict(t, "Validate", e.Validate(), envelope.ErrTooLarge)
		})
	}
	e := resultRecord(t)
	e.Data[field] = within + "\x00"
	checkVerdict(t, "NUL inside budget", e.Validate(), envelope.ErrText)
	e.Data = map[string]any{within + "\x00": true}
	checkVerdict(t, "key NUL inside budget", e.Validate(), envelope.ErrText)
}

func TestValidateDataByteBudgetKeepsWireRecords(t *testing.T) {
	t.Parallel()
	for name, data := range map[string]map[string]any{
		"string":        {"str": strings.Repeat("w", 60000)},
		"shared text":   {"str": slices.Repeat([]string{strings.Repeat("w", 4000)}, 15)},
		"object key":    {strings.Repeat("w", 60000): true},
		"dense ints":    {"values": make([]int, 32000)},
		"dense objects": {"objects": slices.Repeat([]any{map[string]any{}}, 20000)},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			e := resultRecord(t)
			e.Data = data
			checkWireSizedData(t, e)
		})
	}
}

func checkWireSizedData(t *testing.T, e envelope.Envelope) {
	t.Helper()
	wire := mustMarshalJSON(t, e)
	if len(wire) > 64<<10 {
		t.Fatalf("expected wire-sized record, got %d bytes", len(wire))
	}
	checkVerdict(t, "Validate", e.Validate(), nil)
	out, err := envelope.DecodeStrict(wire)
	if err != nil {
		t.Fatal(err)
	}
	checkVerdict(t, "Validate decoded record", out.Validate(), nil)
	if !bytes.Equal(wire, mustMarshalJSON(t, out)) {
		t.Error("wire-sized record changed after round trip")
	}
}

func TestValidateRandomWireSizedTrees(t *testing.T) {
	t.Parallel()
	const limit = 64 << 10
	e := resultRecord(t)
	for seed := range byte(48) {
		e.Data = map[string]any{"tree": randomDataTree(sha256.Sum256([]byte{seed})), "pad": ""}
		padding := limit - len(mustMarshalJSON(t, e)) - int(seed%16)
		e.Data["pad"] = strings.Repeat("p", padding)
		checkWireSizedData(t, e)
	}
	e.Data["pad"] = ""
	e.Data["pad"] = strings.Repeat("p", limit+1-len(mustMarshalJSON(t, e)))
	_, err := envelope.DecodeStrict(mustMarshalJSON(t, e))
	checkVerdict(t, "DecodeStrict above wire limit", err, envelope.ErrTooLarge)
}

func randomDataTree(seed [sha256.Size]byte) any {
	var value any = nil
	atoms := []any{"", "<>&\n\"", "\u00e9\u4e2d", []byte{0, 1, 255}, json.Number("42e+7"), 0, true}
	for i := range 1 + int(seed[0]%8) {
		atom := atoms[int(seed[1+2*i])%len(atoms)]
		switch seed[2+2*i] % 3 {
		case 0:
			value = map[string]any{"\u00e9\n": atom, "": value}
		case 1:
			value = []any{value, atom, value}
		default:
			value = [2]any{atom, value}
		}
	}
	return value
}

func BenchmarkValidateRepeatedLargeString(b *testing.B) {
	e := repeatedLargeString(b)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if err := e.Validate(); err != nil && !errors.Is(err, envelope.ErrTooLarge) {
			b.Fatal(err)
		}
	}
}

func repeatedLargeString(tb testing.TB) envelope.Envelope {
	tb.Helper()
	e := resultRecord(tb)
	e.Data["text"] = slices.Repeat([]string{strings.Repeat("a", 1<<20)}, 16000)
	return e
}
