package envelope_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/devsy-org/devsy-signals/envelope"
)

const oneRecord = `{"v":1}`

func recordOfSize(size int) string {
	const prefix = `{"v":1,"msg":"`
	const suffix = `"}`
	return prefix + strings.Repeat("x", size-len(prefix)-len(suffix)) + suffix
}

func TestDecodeSharedRules(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		want error
	}{
		{"exact size", recordOfSize(65536), nil},
		{"one byte over", recordOfSize(65537), envelope.ErrTooLarge},
		{"oversize checked first", strings.Repeat("\x00", 65537), envelope.ErrTooLarge},
		{"malformed", `{"v":1,}`, envelope.ErrNotRecord},
		{"truncated", `{"v":1`, envelope.ErrNotRecord},
		{"array", `[1]`, envelope.ErrNotRecord},
		{"trailing data", oneRecord + oneRecord, envelope.ErrNotRecord},
		{"duplicate", `{"v":1,"\u0076":1}`, envelope.ErrNotRecord},
		{"duplicate in unknown", `{"v":1,"next":[{"a":null,"a":2}]}`, envelope.ErrNotRecord},
		{"duplicate in data", `{"v":1,"data":{"list":[{"a":1,"a":2}]}}`, envelope.ErrNotRecord},
		{"missing version", `{}`, envelope.ErrVersion},
		{"null version", `{"v":null}`, envelope.ErrVersion},
		{"major version", `{"v":2}`, envelope.ErrVersion},
		{"quoted version", `{"v":"1"}`, envelope.ErrVersion},
		{"fractional version", `{"v":1.5}`, envelope.ErrVersion},
		{"decimal one", `{"v":1.0}`, nil},
		{"scientific one", `{"v":1e0}`, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			checkBoth(t, c.in, c.want)
		})
	}
}

func TestDecodeText(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		want error
	}{
		{"bad UTF-8", "{\"v\":1,\"msg\":\"\xff\"}", envelope.ErrText},
		{"raw surrogate bytes", "{\"v\":1,\"msg\":\"\xed\xa0\x80\"}", envelope.ErrText},
		{"raw NUL", "{\"v\":1,\"msg\":\"\x00\"}", envelope.ErrText},
		{"escaped NUL key", `{"v":1,"next\u0000":true}`, envelope.ErrText},
		{"escaped NUL value", `{"v":1,"next":["\u0000"]}`, envelope.ErrText},
		{"lone high surrogate", `{"v":1,"msg":"\ud800"}`, envelope.ErrText},
		{"lone low surrogate", `{"v":1,"msg":"\udc00"}`, envelope.ErrText},
		{"reversed surrogates", `{"v":1,"msg":"\udc00\ud800"}`, envelope.ErrText},
		{"two high surrogates", `{"v":1,"msg":"\ud800\ud800"}`, envelope.ErrText},
		{"valid pair", `{"v":1,"msg":"\ud83d\uDE00"}`, nil},
		{"raw Unicode", "{\"v\":1,\"msg\":\"\xf0\x9f\x98\x80\"}", nil},
		{"escaped backslash", `{"v":1,"msg":"\\ud800 \\u0000"}`, nil},
		{"malformed escape", `{"v":1,"msg":"\uXYZ0"}`, envelope.ErrNotRecord},
		{"escape outside string", `{"v":1,"msg":\u0000}`, envelope.ErrNotRecord},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			checkBoth(t, c.in, c.want)
		})
	}
}

func TestDecodeKeepsSurrogatePair(t *testing.T) {
	t.Parallel()
	in := []byte(`{"v":1,"msg":"\ud83d\ude00"}`)
	e, err := envelope.DecodeStrict(in)
	if err != nil || e.Msg != "\U0001f600" {
		t.Fatalf("got %q, %v", e.Msg, err)
	}
}

func TestDecodeDeepArrays(t *testing.T) {
	t.Parallel()
	const depth = 11000
	in := `{"v":1,"data":{"a":` + strings.Repeat(
		"[",
		depth,
	) + "0" + strings.Repeat(
		"]",
		depth,
	) + "}}"
	checkBoth(t, in, nil)
}

func checkBoth(t *testing.T, in string, want error) {
	t.Helper()
	for name, decode := range map[string]func([]byte) (envelope.Envelope, error){
		"Decode": envelope.Decode, "DecodeStrict": envelope.DecodeStrict,
	} {
		got, err := decode([]byte(in))
		if !errors.Is(err, want) {
			t.Errorf("%s: got %v, want %v", name, err, want)
		} else if err == nil && got.V != 1 {
			t.Errorf("%s: decoded version %d, want 1", name, got.V)
		}
	}
}

type fieldCase struct {
	name   string
	in     string
	absent func(envelope.Envelope) bool
}

func TestDecodeToleratesUnknownFields(t *testing.T) {
	t.Parallel()
	checkToleratedFields(t, []fieldCase{
		{
			"new top-level field",
			`{"v":1,"future":1}`,
			func(e envelope.Envelope) bool { return e.Msg == "" },
		},
		{"new actor field", `{"v":1,"actor":{"future":1}}`, func(e envelope.Envelope) bool {
			return e.Actor != nil && *e.Actor == (envelope.Actor{})
		}},
		{"new error field", errorWith(errBody + `,"future":1`), func(e envelope.Envelope) bool {
			return e.Error != nil && e.Error.Hint == ""
		}},
		{
			"folded top-level field",
			`{"v":1,"Msg":"hidden"}`,
			func(e envelope.Envelope) bool { return e.Msg == "" },
		},
		{"folded actor field", `{"v":1,"actor":{"ID":"hidden"}}`, func(e envelope.Envelope) bool {
			return e.Actor != nil && e.Actor.ID == ""
		}},
		{
			"folded error field",
			errorWith(errBody + `,"Hint":"hidden"`),
			func(e envelope.Envelope) bool {
				return e.Error != nil && e.Error.Hint == ""
			},
		},
	})
}

func TestDecodeToleratesWrongTypesAndNulls(t *testing.T) {
	t.Parallel()
	checkToleratedFields(t, []fieldCase{
		{
			"null top-level field",
			`{"v":1,"actor":null}`,
			func(e envelope.Envelope) bool { return e.Actor == nil },
		},
		{
			"null context entry",
			errorWith(errBody + `,"context":{"k":null}`),
			func(e envelope.Envelope) bool {
				return e.Error != nil && len(e.Error.Context) == 0
			},
		},
		{
			"wrong field types",
			`{"v":1,"msg":true,"actor":[],"attrs":12}`,
			func(e envelope.Envelope) bool {
				return e.Msg == "" && e.Actor == nil && e.Attrs == nil
			},
		},
	})
	t.Run("opaque null", func(t *testing.T) {
		t.Parallel()
		checkTolerantOpaqueNull(t)
	})
}

func checkTolerantOpaqueNull(t *testing.T) {
	t.Helper()
	e, err := envelope.Decode([]byte(`{"v":1,"data":{"a":null,"b":[null]}}`))
	if err != nil {
		t.Fatal(err)
	}
	nullValue, present := e.Data["a"]
	items, ok := e.Data["b"].([]any)
	if !present || nullValue != nil || !ok || len(items) != 1 || items[0] != nil {
		t.Fatalf("got data %#v, want a present as nil and b containing one nil", e.Data)
	}
}

func TestDecodeStrictPreservesOpaqueNulls(t *testing.T) {
	t.Parallel()
	const payload = `{"a":null,"b":[1,null,2]}`
	for _, field := range []string{"data", "attrs"} {
		t.Run(field, func(t *testing.T) {
			t.Parallel()
			in := fmt.Sprintf(`{"v":1,%q:%s}`, field, payload)
			e, err := envelope.DecodeStrict([]byte(in))
			if err != nil {
				t.Fatal(err)
			}
			got := e.Data
			if field == "attrs" {
				got = e.Attrs
			}
			encoded, err := json.Marshal(got)
			if err != nil || string(encoded) != payload {
				t.Fatalf("got %s, %v; want %s", encoded, err, payload)
			}
		})
	}
}

func TestDecodeStrictRejectsTypedNullsBesideOpaqueNulls(t *testing.T) {
	t.Parallel()
	for field, in := range map[string]string{
		"envelope.actor":           `{"v":1,"actor":null,"data":{"n":null}}`,
		"envelope.error.context.k": `{"v":1,"attrs":{"n":null},"error":{` + errBody + `,"context":{"k":null}}}`,
	} {
		t.Run(field, func(t *testing.T) {
			t.Parallel()
			_, err := envelope.DecodeStrict([]byte(in))
			if !errors.Is(err, envelope.ErrStrict) || !strings.Contains(err.Error(), field) {
				t.Fatalf("got %v, want ErrStrict for %s", err, field)
			}
		})
	}
}

func checkToleratedFields(t *testing.T, cases []fieldCase) {
	t.Helper()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			e := tolerantOnly(t, c.in)
			if !c.absent(e) {
				t.Errorf("field not treated as absent: %+v", e)
			}
		})
	}
}

func TestDecodeToleratesUnknownEnums(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in    string
		known func(envelope.Envelope) (string, bool)
	}{
		{`{"v":1,"kind":"future"}`, func(e envelope.Envelope) (string, bool) {
			return string(e.Kind), e.Kind.Known()
		}},
		{`{"v":1,"src":"future"}`, func(e envelope.Envelope) (string, bool) {
			return string(e.Src), e.Src.Known()
		}},
		{`{"v":1,"actor":{"kind":"future"}}`, func(e envelope.Envelope) (string, bool) {
			return string(e.Actor.Kind), e.Actor.Kind.Known()
		}},
		{`{"v":1,"level":"future"}`, func(e envelope.Envelope) (string, bool) {
			return string(e.Level), e.Level.Known()
		}},
		{errorWith(`"class":"future","retryable":false`), func(e envelope.Envelope) (string, bool) {
			return string(e.Error.Class), e.Error.Class.Known()
		}},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			t.Parallel()
			if got, known := c.known(tolerantOnly(t, c.in)); got != "future" || known {
				t.Errorf("got enum %q, Known() = %t", got, known)
			}
		})
	}
}

func TestDecodeStrictRejectsEmptyOptionalStrings(t *testing.T) {
	t.Parallel()
	for _, name := range []string{
		"type", "id", "level", "msg", "traceId", "operationId", "parentOperationId",
		"workspace", "session", "pool", "host", "policyVersion",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			tolerantOnly(t, fmt.Sprintf(`{"v":1,%q:""}`, name))
		})
	}
	t.Run("error.hint", func(t *testing.T) {
		t.Parallel()
		tolerantOnly(t, errorWith(errBody+`,"hint":""`))
	})
}

func TestDecodeStrictAcceptsKnownProviderSource(t *testing.T) {
	t.Parallel()
	checkBoth(t, `{"v":1,"src":"provider/docker-v2.1_arm"}`, nil)
}

func tolerantOnly(t *testing.T, in string) envelope.Envelope {
	t.Helper()
	e, err := envelope.Decode([]byte(in))
	if err != nil {
		t.Fatalf("tolerant decode rejected %s: %v", in, err)
	}
	if _, err := envelope.DecodeStrict([]byte(in)); !errors.Is(err, envelope.ErrStrict) {
		t.Errorf("strict decode: got %v, want %v", err, envelope.ErrStrict)
	}
	return e
}

func FuzzDecode(f *testing.F) {
	for _, seed := range []string{
		oneRecord, logWith(""), errorWith(errBody), `{"v":1,"data":{"a":[1,2]}}`,
		`{"v":1,"msg":"\ud83d\ude00"}`, `{"v":1,"msg":"\ud800"}`, `{"v":1,"v":1}`,
	} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, in []byte) {
		_, err := envelope.Decode(in)
		if err == nil && len(in) < 10000 && !json.Valid(in) {
			t.Fatal("accepted malformed JSON")
		}
		if _, strictErr := envelope.DecodeStrict(in); strictErr == nil && err != nil {
			t.Fatalf("strict accepted, tolerant rejected: %v", err)
		}
	})
}
