package envelope_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/devsy-org/devsy-signals/envelope"
)

// Field values below are nested k levels deep, so the whole record is k+1
// levels deep. The limit is 64 levels counting the top-level object as 1.
const (
	deepestAccepted = 63
	shallowestError = 64
	hugeArrays      = 11000
	hugeObjects     = 10000 // the record size limit stops objects short of 11000

	fieldData   = "data"
	fieldAttrs  = "attrs"
	fieldFuture = "future"
	shapeObject = "objects"
	shapeArray  = "arrays"
	leafNull    = "null"
)

type decodeFunc func([]byte) (envelope.Envelope, error)

func decoders() map[string]decodeFunc {
	return map[string]decodeFunc{"Decode": envelope.Decode, "DecodeStrict": envelope.DecodeStrict}
}

func objects(k int, leaf string) string {
	return strings.Repeat(`{"a":`, k) + leaf + strings.Repeat("}", k)
}

func arrays(k int, leaf string) string {
	return strings.Repeat("[", k) + leaf + strings.Repeat("]", k)
}

// shapes returns field values k levels deep; data and attrs must be objects.
func shapes(k int, leaf string) map[string]string {
	return map[string]string{
		shapeObject: objects(k, leaf),
		shapeArray:  `{"a":` + arrays(k-1, leaf) + "}",
	}
}

// checkDepth expects ErrDepth from both decoders when want is ErrDepth. An
// accepted unknown field is still an error in DecodeStrict.
func checkDepth(t *testing.T, in, field string, want error) {
	t.Helper()
	if want == nil && field == fieldFuture {
		if _, err := envelope.Decode([]byte(in)); err != nil {
			t.Errorf("Decode: %v", err)
		}
		if _, err := envelope.DecodeStrict([]byte(in)); !errors.Is(err, envelope.ErrStrict) {
			t.Errorf("DecodeStrict: got %v, want ErrStrict", err)
		}
		return
	}
	checkBoth(t, in, want)
}

func TestDecodeDepthBoundary(t *testing.T) {
	t.Parallel()
	type depthCase struct {
		k    int
		want error
	}
	cases := map[string][]depthCase{
		shapeObject: {
			{deepestAccepted, nil},
			{shallowestError, envelope.ErrDepth},
			{hugeObjects, envelope.ErrDepth},
		},
		shapeArray: {
			{deepestAccepted, nil},
			{shallowestError, envelope.ErrDepth},
			{hugeArrays, envelope.ErrDepth},
		},
	}
	for _, field := range []string{fieldData, fieldAttrs, fieldFuture} {
		for _, leaf := range []string{"0", leafNull} {
			for shape, list := range cases {
				for _, c := range list {
					name := fmt.Sprintf("%s/%s/%s/%d", field, shape, leaf, c.k)
					in := `{"v":1,"` + field + `":` + shapes(c.k, leaf)[shape] + "}"
					t.Run(name, func(t *testing.T) {
						t.Parallel()
						checkDepth(t, in, field, c.want)
					})
				}
			}
		}
	}
}

func TestDecodeDepthPlainArraysAndObjects(t *testing.T) {
	t.Parallel()
	for name, wrap := range map[string]func(int) string{
		shapeArray:  func(k int) string { return arrays(k, "1") },
		shapeObject: func(k int) string { return objects(k, "1") },
	} {
		for _, c := range []struct {
			k    int
			want error
		}{{deepestAccepted, nil}, {shallowestError, envelope.ErrDepth}} {
			t.Run(fmt.Sprintf("%s/%d", name, c.k), func(t *testing.T) {
				t.Parallel()
				checkDepth(t, `{"v":1,"future":`+wrap(c.k)+"}", fieldFuture, c.want)
			})
		}
	}
}

func TestDecodeDepthErrorHasNoContent(t *testing.T) {
	t.Parallel()
	in := `{"v":1,"data":` + objects(shallowestError, `"secret-value"`) + "}"
	for name, decode := range decoders() {
		_, err := decode([]byte(in))
		if !errors.Is(err, envelope.ErrDepth) || errors.Is(err, envelope.ErrNotRecord) {
			t.Fatalf("%s: got %v, want only ErrDepth", name, err)
		}
		if !strings.Contains(err.Error(), "64") || strings.Contains(err.Error(), "secret") {
			t.Errorf("%s: message %q must name the limit and no content", name, err)
		}
	}
}

func TestDecodeDepthResetsAfterSiblings(t *testing.T) {
	t.Parallel()
	deep := arrays(deepestAccepted-2, "0")
	in := `{"v":1,"data":{"a":` + deep + `,"b":` + deep + `,"c":[` + deep + "," + deep + "]}}"
	checkBoth(t, in, nil)
}

func TestDecodeDepthWideShallowRecord(t *testing.T) {
	t.Parallel()
	var members []string
	for i := range 700 {
		members = append(members, fmt.Sprintf(`"k%d":[%d,{"x":null}]`, i, i))
	}
	wide := "{" + strings.Join(members, ",") + "}"
	list := "[" + strings.Repeat("[0],", 3000) + "0]"
	in := `{"v":1,"data":` + wide + `,"attrs":{"list":` + list + `},"future":` + wide + "}"
	if _, err := envelope.Decode([]byte(in)); err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if _, err := envelope.DecodeStrict(
		[]byte(strings.Replace(in, `,"future":`+wide, "", 1)),
	); err != nil {
		t.Fatalf("DecodeStrict: %v", err)
	}
}

func TestDecodeDepthRoundTrip(t *testing.T) {
	t.Parallel()
	for shape, value := range shapes(deepestAccepted, leafNull) {
		t.Run(shape, func(t *testing.T) {
			t.Parallel()
			in := resultWith(`,"data":` + value + `,"attrs":` + value)
			e, err := envelope.DecodeStrict([]byte(in))
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			if _, err := json.Marshal(e); err != nil {
				t.Fatalf("json.Marshal: %v", err)
			}
			checkMarshalRoundTrip(t, e)
		})
	}
}

func checkMarshalRoundTrip(t *testing.T, e envelope.Envelope) {
	t.Helper()
	wire, err := e.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}
	back, err := envelope.DecodeStrict(wire)
	if err != nil {
		t.Fatalf("decode marshaled record: %v", err)
	}
	if again, err := back.MarshalJSON(); err != nil || string(again) != string(wire) {
		t.Errorf("round trip changed the record: %v", err)
	}
}

func TestDecodeDepthRejectionIsFast(t *testing.T) {
	t.Parallel()
	in := []byte(`{"v":1,"data":` + objects(hugeObjects, "0") + "}")
	start := time.Now()
	for range 200 {
		if _, err := envelope.Decode(in); !errors.Is(err, envelope.ErrDepth) {
			t.Fatalf("got %v, want ErrDepth", err)
		}
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("200 rejections took %v", elapsed)
	}
}

func BenchmarkDecodeDepthRejection(b *testing.B) {
	in := []byte(`{"v":1,"data":` + objects(hugeObjects, "0") + "}")
	b.ReportAllocs()
	for range b.N {
		if _, err := envelope.Decode(in); err == nil {
			b.Fatal("accepted")
		}
	}
}
