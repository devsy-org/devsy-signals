package envelope_test

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/devsy-org/devsy-signals/envelope"
	"github.com/devsy-org/devsy-signals/schema"
)

func resultRecord(tb testing.TB) envelope.Envelope {
	tb.Helper()
	e, err := envelope.DecodeStrict([]byte(resultWith(`,"data":{}`)))
	if err != nil {
		tb.Fatal(err)
	}
	return e
}

func TestValidateDataWireText(t *testing.T) {
	t.Parallel()
	const marker = "wire-text-marker"
	for name, data := range map[string]func(string) map[string]any{
		"key": func(text string) map[string]any {
			return map[string]any{"list": []any{map[string]any{text: "leaf"}}}
		},
		"value": func(text string) map[string]any {
			return map[string]any{"list": [1]map[string]string{{"key": text}}}
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			e := resultRecord(t)
			e.Data = data(marker)
			raw := bytes.ReplaceAll(mustMarshalJSON(t, e), []byte(marker), []byte("text\xff"))
			_, err := envelope.DecodeStrict(raw)
			checkVerdict(t, "invalid raw bytes", err, envelope.ErrText)
			e.Data = data("text\xff")
			checkVerdict(t, "Validate invalid UTF-8", e.Validate(), envelope.ErrText)
			// encoding/json replaces malformed UTF-8, so the marshaled wire text differs.
			_, err = envelope.DecodeStrict(mustMarshalJSON(t, e))
			checkVerdict(t, "DecodeStrict after UTF-8 replacement", err, nil)
			e.Data = data("text\x00")
			checkDataRoundTripVerdict(t, e, envelope.ErrText)
			e.Data = data("literal <>&")
			checkDataRoundTripVerdict(t, e, nil)
		})
	}
}

func checkDataRoundTripVerdict(t *testing.T, e envelope.Envelope, want error) {
	t.Helper()
	checkVerdict(t, "Validate", e.Validate(), want)
	_, err := envelope.DecodeStrict(mustMarshalJSON(t, e))
	checkVerdict(t, "DecodeStrict after marshal", err, want)
}

func checkVerdict(t *testing.T, step string, got, want error) {
	t.Helper()
	if !errors.Is(got, want) {
		t.Errorf("%s: got %v, want %v", step, got, want)
	}
}

func TestValidateDataDepth(t *testing.T) {
	t.Parallel()
	for _, n := range []int{schema.MaxRecordDepth, schema.MaxRecordDepth + 1} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			t.Parallel()
			e := resultRecord(t)
			var value any = "leaf"
			for range n - 2 {
				value = []any{value}
			}
			e.Data["list"] = value
			var want error
			if n > schema.MaxRecordDepth {
				want = envelope.ErrDepth
			}
			checkDataRoundTripVerdict(t, e, want)
		})
	}
}

func TestValidateDataCycle(t *testing.T) {
	t.Parallel()
	e := resultRecord(t)
	e.Data["self"] = e.Data
	checkVerdict(t, "Validate cyclic map", e.Validate(), envelope.ErrDepth)

	var ptr any
	ptr = &ptr
	e.Data["self"] = ptr
	checkVerdict(t, "Validate cyclic pointer", e.Validate(), envelope.ErrDepth)

	slice := make([]any, 1)
	slice[0] = slice
	e.Data["self"] = slice
	checkVerdict(t, "Validate cyclic slice", e.Validate(), envelope.ErrDepth)
}

type textMarshalerString string

func (textMarshalerString) MarshalText() ([]byte, error) { return []byte("text\x00"), nil }

type jsonMarshalerString string

func (*jsonMarshalerString) MarshalJSON() ([]byte, error) { return []byte(`"text\u0000"`), nil }

type plainWireString string

type marshalerList []string

func (marshalerList) MarshalJSON() ([]byte, error) { return []byte(`["text\u0000"]`), nil }

func TestValidateRejectsWireMarshalers(t *testing.T) {
	t.Parallel()
	const safe = "safe"
	for _, tc := range []struct {
		name  string
		value any
		want  error
	}{
		{"text marshaler", textMarshalerString(safe), envelope.ErrText},
		{"pointer JSON marshaler in slice", []jsonMarshalerString{safe}, envelope.ErrText},
		{"slice with custom marshaler", marshalerList{safe}, envelope.ErrText},
		{"plain named string", plainWireString(safe), nil},
		{"plain named string in array", [1]plainWireString{safe}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e := resultRecord(t)
			const field = "payload"
			e.Data[field] = tc.value
			checkDataRoundTripVerdict(t, e, tc.want)
			delete(e.Data, field)
			e.Attrs = map[string]any{field: tc.value}
			checkDataRoundTripVerdict(t, e, tc.want)
		})
	}
}
