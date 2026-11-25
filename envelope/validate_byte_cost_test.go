package envelope

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"
)

func TestDataChargeIsWireLowerBound(t *testing.T) {
	t.Parallel()
	shared := []any{"z", json.Number("123")}
	for name, value := range map[string]any{
		"escaping":   "\n\"<>&\u2028",
		"unicode":    "\u00e9\u4e2d",
		"empty":      "",
		"keys":       map[string]int{"\n\u00e9": 0, "": 1},
		"bytes":      []byte{0, 255},
		"number":     json.Number("0"),
		"exponent":   json.Number("1e+400"),
		"containers": []any{map[string]any{"x": shared}, []any{shared}, [1]int{1}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assertDataWireCharge(t, value)
		})
	}
}

func assertDataWireCharge(t *testing.T, value any) {
	t.Helper()
	checks := dataChecks{seen: make(map[dataVisit]int), remaining: maxRecordBytes}
	if err := checks.valueErr(reflect.ValueOf(value), 2); err != nil {
		t.Fatal(err)
	}
	var wire bytes.Buffer
	enc := json.NewEncoder(&wire)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(value); err != nil {
		t.Fatal(err)
	}
	if cost, size := maxRecordBytes-checks.remaining, wire.Len()-1; cost > size {
		t.Errorf("charge %d exceeds %d emitted bytes", cost, size)
	}
}
