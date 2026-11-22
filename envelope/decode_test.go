package envelope_test

import (
	"encoding/json"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/devsy-org/devsy-signals/envelope"
)

const (
	logFields = `"kind":"log","ts":"2026-11-22T10:15:03.123Z","src":"cli","level":"info","msg":"m"`
	resFields = `"kind":"result","type":"a.b","id":"01JD8Z3K5V7Q2M4N6P8R0S1T2W",` +
		`"ts":"2026-11-22T10:15:03.123Z","src":"cli"`
	errFields = `"kind":"error","type":"a.b","id":"01JD8Z3K5V7Q2M4N6P8R0S1T2W",` +
		`"ts":"2026-11-22T10:15:03.123Z","src":"cli"`
	errBody = `"code":"a.b","message":"m","class":"internal","retryable":false`
)

func logWith(extra string) string { return `{"v":1,` + logFields + extra + "}" }

func resultWith(extra string) string { return `{"v":1,` + resFields + extra + "}" }

func errorWith(body string) string {
	return `{"v":1,` + errFields + `,"error":{` + body + "}}"
}

func TestDecodeRejects(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"null root":             `null`,
		"array root":            `[]`,
		"string root":           `"x"`,
		"trailing data":         logWith("") + " {}",
		"duplicate kind":        logWith(`,"kind":"status"`),
		"duplicate data key":    resultWith(`,"data":{"outcome":"a","outcome":"b"}`),
		"duplicate nested":      resultWith(`,"data":{"a":[{"x":1,"x":2}]}`),
		"duplicate attrs key":   logWith(`,"attrs":{"a":1,"a":2}`),
		"duplicate error field": errorWith(errBody + `,"code":"x.y"`),
		"case folded v":         `{"V":1,` + logFields + "}",
		"case folded msg":       logWith(`,"Msg":"x"`),
		"retryable folded":      errorWith(errBody + `,"Retryable":true`),
		"case folded actor":     logWith(`,"actor":{"kind":"user","ID":"x"}`),
		"null actor":            logWith(`,"actor":null`),
		"null error":            logWith(`,"error":null`),
		"null data":             resultWith(`,"data":null`),
		"null hint":             errorWith(errBody + `,"hint":null`),
		"null code": errorWith(
			`"code":null,"message":"m","class":"internal","retryable":false`,
		),
		"null message": errorWith(
			`"code":"a.b","message":null,"class":"internal","retryable":false`,
		),
		"null class": errorWith(
			`"code":"a.b","message":"m","class":null,"retryable":false`,
		),
		"null retryable": errorWith(
			`"code":"a.b","message":"m","class":"internal","retryable":null`,
		),
		"null context":       errorWith(errBody + `,"context":null`),
		"null context value": errorWith(errBody + `,"context":{"k":null}`),
		"v is two":           `{"v":2,` + logFields + "}",
		"v is fractional":    `{"v":1.5,` + logFields + "}",
		"v is string":        `{"v":"1",` + logFields + "}",
		"v is huge exponent": `{"v":1e999999999,` + logFields + "}",
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if e, err := envelope.Decode([]byte(in)); err == nil {
				t.Fatalf("accepted as %+v", e)
			}
		})
	}
}

func TestDecodeNormalizesV(t *testing.T) {
	t.Parallel()
	zeros := strings.Repeat("0", 40)
	for _, v := range []string{
		"1", "1.0", "1e0", "10e-1", "0.1e1", "100e-2", "1.00E+0", "0.00001e5", "1E+0", "10E-01",
		"1." + zeros, "1" + zeros + "e-40", "0." + zeros + "1e41", "1e" + zeros, "1e-" + zeros,
		"1" + "e00000000000000000000",
	} {
		e, err := envelope.Decode([]byte(`{"v":` + v + "," + logFields + "}"))
		if err != nil || e.V != 1 {
			t.Errorf("v=%s: %+v %v", v, e, err)
		}
	}
}

func TestDecodeRejectsOtherV(t *testing.T) {
	t.Parallel()
	for _, v := range []string{
		"0", "-0", "-1", "2", "1.5", "0.9999999999", "10", "1e1", "1e-1", "1e" + strings.Repeat("0", 30) + "1",
		"1e999999", "1e-999999", "1e99999999999999999999", "1" + strings.Repeat("0", 5000), "true", `"1"`, "[1]", "{}",
	} {
		if e, err := envelope.Decode([]byte(`{"v":` + v + "," + logFields + "}")); err == nil {
			t.Errorf("v=%.40s accepted as %+v", v, e)
		}
	}
}

//nolint:paralleltest // reads process-wide runtime.MemStats, parallel tests would skew the byte count
func TestDecodeHugeExponentIsCheap(t *testing.T) {
	in := []byte(`{"v":1e999999,` + logFields + "}")
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	for range 20 {
		if _, err := envelope.Decode(in); err == nil {
			t.Fatal("accepted")
		}
	}
	runtime.ReadMemStats(&after)
	if perCall := (after.TotalAlloc - before.TotalAlloc) / 20; perCall > 64<<10 {
		t.Fatalf("%d bytes allocated per call", perCall)
	}
}

func BenchmarkDecodeNumericArray(b *testing.B) {
	var sb strings.Builder
	sb.WriteString(logWith(""))
	record := strings.TrimSuffix(sb.String(), "}") + `,"data":{"values":[`
	sb.Reset()
	sb.WriteString(record)
	for i := 0; sb.Len() < 1<<20; i++ {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(strconv.Itoa(i))
	}
	sb.WriteString("]}}")
	data := []byte(sb.String())
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, err := envelope.Decode(data); err != nil {
			b.Fatal(err)
		}
	}
}

func TestDecodeKeepsNumbersExact(t *testing.T) {
	t.Parallel()
	e, err := envelope.Decode(
		[]byte(logWith(`,"attrs":{"a":1e400,"b":12345678901234567890},"data":{"c":1e400}`)),
	)
	if err != nil {
		t.Fatal(err)
	}
	if got := e.Attrs["a"]; got != json.Number("1e400") {
		t.Fatalf("attrs.a = %#v", got)
	}
	if got := e.Attrs["b"]; got != json.Number("12345678901234567890") {
		t.Fatalf("attrs.b = %#v", got)
	}
	if err := e.Validate(); err != nil {
		t.Fatal(err)
	}
}
