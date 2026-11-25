package envelope_test

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/devsy-org/devsy-signals/codes"
	"github.com/devsy-org/devsy-signals/envelope"
)

func fixtures(t *testing.T, dir string) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join("..", "fixtures", "envelope", dir, "*.json"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no fixtures in %s: %v", dir, err)
	}
	return files
}

func decodeFile(t *testing.T, path string) (envelope.Envelope, error) {
	t.Helper()
	data, err := os.ReadFile(path) //nolint:gosec // fixture path
	if err != nil {
		t.Fatal(err)
	}
	return envelope.DecodeStrict(data)
}

func TestValidFixtures(t *testing.T) {
	t.Parallel()
	for _, path := range fixtures(t, "valid") {
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()
			e, err := decodeFile(t, path)
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			if err := e.Validate(); err != nil {
				t.Fatalf("validate: %v", err)
			}
		})
	}
}

func TestInvalidFixtures(t *testing.T) {
	t.Parallel()
	for _, path := range fixtures(t, "invalid") {
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()
			e, err := decodeFile(t, path)
			if err != nil {
				return
			}
			if err := e.Validate(); err == nil {
				t.Fatal("expected a validation error")
			}
		})
	}
}

func TestRoundTrip(t *testing.T) {
	t.Parallel()
	for _, path := range fixtures(t, "valid") {
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()
			e, err := decodeFile(t, path)
			if err != nil {
				t.Fatal(err)
			}
			out := mustMarshal(t, e)
			back := mustDecodeValid(t, out)
			if again := mustMarshal(t, back); string(again) != string(out) {
				t.Fatalf("marshal is not stable:\n%s\n%s", out, again)
			}
		})
	}
}

func mustMarshal(t *testing.T, e envelope.Envelope) []byte {
	t.Helper()
	out, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func mustDecodeValid(t *testing.T, data []byte) envelope.Envelope {
	t.Helper()
	e, err := envelope.DecodeStrict(data)
	if err != nil {
		t.Fatalf("decode marshaled: %v\n%s", err, data)
	}
	if err := e.Validate(); err != nil {
		t.Fatalf("validate marshaled: %v\n%s", err, data)
	}
	return e
}

func TestValidateEmptyData(t *testing.T) {
	t.Parallel()
	if err := statusWithEmptyData().Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestValidateDecodedExactNumbers(t *testing.T) {
	t.Parallel()
	mustDecodeValid(
		t,
		[]byte(logWith(`,"attrs":{"a":1e400,"b":12345678901234567890},"data":{"c":1e400}`)),
	)
}

func TestValidateReportsEveryProblem(t *testing.T) {
	t.Parallel()
	err := envelope.Envelope{}.Validate()
	if err == nil {
		t.Fatal("expected errors")
	}
	for _, want := range []string{"v must be 1", "kind", "ts", "src"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("missing %q in %v", want, err)
		}
	}
}

type attrCase struct {
	attrs map[string]any
	ok    bool
}

func runAttrCases(t *testing.T, cases map[string]attrCase) {
	t.Helper()
	base := envelope.Envelope{
		V: 1, Kind: envelope.KindLog, TS: envelope.FormatTime(time.Now()), Src: srcCLI,
		Level: envelope.LevelInfo, Msg: "m",
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			e := base
			e.Attrs = c.attrs
			if err := e.Validate(); (err == nil) != c.ok {
				t.Fatalf("ok=%v, err=%v", c.ok, err)
			}
		})
	}
}

func TestValidateAttrsFromGo(t *testing.T) {
	t.Parallel()
	cases := map[string]attrCase{
		"scalars": {
			map[string]any{"a": "x", "b.c": 1, "d": 2.5, "e": true, "f": int64(3)},
			true,
		},
		"string array":  {map[string]any{"a": []string{"x", "y"}}, true},
		"nested array":  {map[string]any{"a": []any{[]any{"x"}}}, false},
		"map value":     {map[string]any{"a": map[string]any{}}, false},
		"nil value":     {map[string]any{"a": nil}, false},
		"long bytes":    {map[string]any{"a": strings.Repeat("x", 4097)}, false},
		"limit bytes":   {map[string]any{"a": strings.Repeat("x", 4096)}, true},
		"multibyte":     {map[string]any{"a": strings.Repeat("\u00e9", 2049)}, false},
		"key too long":  {map[string]any{strings.Repeat("a", 129): "x"}, false},
		"key at limit":  {map[string]any{strings.Repeat("a", 128): "x"}, true},
		"empty segment": {map[string]any{"a..b": "x"}, false},
		"int slice":     {map[string]any{"a": []int{1, 2}}, true},
		"float slice":   {map[string]any{"a": []float64{1.5}}, true},
		"bool slice":    {map[string]any{"a": []bool{true}}, true},
		"int array":     {map[string]any{"a": [2]int{1, 2}}, true},
		"empty slice":   {map[string]any{"a": []string{}}, true},
		"nil slice":     {map[string]any{"a": []string(nil)}, false},
		"byte slice":    {map[string]any{"a": []byte("x")}, false},
		"long in slice": {map[string]any{"a": []string{strings.Repeat("x", 4097)}}, false},
		"map slice":     {map[string]any{"a": []map[string]any{{}}}, false},
		"NaN":           {map[string]any{"a": math.NaN()}, false},
		"+Inf":          {map[string]any{"a": math.Inf(1)}, false},
		"-Inf in slice": {map[string]any{"a": []float64{math.Inf(-1)}}, false},
		"float32 NaN":   {map[string]any{"a": float32(math.NaN())}, false},
		"named string":  {map[string]any{"a": myString("x")}, true},
	}
	runAttrCases(t, cases)
}

func TestValidateNumbersFromGo(t *testing.T) {
	t.Parallel()
	cases := map[string]attrCase{
		"json.Number":   {map[string]any{"a": json.Number("1e400")}, true},
		"bad Number":    {map[string]any{"a": json.Number("abc")}, false},
		"Number true":   {map[string]any{"a": json.Number("true")}, false},
		"Number array":  {map[string]any{"a": json.Number("[]")}, false},
		"Number spaces": {map[string]any{"a": json.Number(" 1 ")}, false},
		"Number string": {map[string]any{"a": json.Number(`"1"`)}, false},
		"Number dot":    {map[string]any{"a": json.Number("1.")}, false},
		"Number plus":   {map[string]any{"a": json.Number("+1")}, false},
		"Number zeros":  {map[string]any{"a": json.Number("01")}, false},
		"Number bare e": {map[string]any{"a": json.Number("1e")}, false},
		"Number empty":  {map[string]any{"a": json.Number("")}, false},
		"Number forms": {
			map[string]any{"a": []json.Number{"-0", "0.5", "1E-5", "10", "1e+400"}},
			true,
		},
	}
	runAttrCases(t, cases)
}

type myString string

func TestErrorClassMatchesRegistry(t *testing.T) {
	t.Parallel()
	e := envelope.Envelope{
		V:    1,
		Kind: envelope.KindError,
		Type: "a.b",
		ID:   envelope.NewID(),
		TS:   envelope.FormatTime(time.Unix(0, 0)),
		Src:  srcCLI,
		Error: &envelope.Error{
			Code:    codes.Timeout.String(),
			Message: "m",
			Class:   codes.ClassTimeout,
		},
	}
	if err := e.Validate(); err != nil {
		t.Fatal(err)
	}
}
