package tracectx_test

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/devsy-org/devsy-signals/tracectx"
)

const (
	example = "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01"
	trace   = "0af7651916cd43dd8448eb211c80319c"
	span    = "b7ad6b7169203331"
)

func TestParseSpecExamples(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		in    string
		flags byte
	}{
		"sampled":        {example, 1},
		"not sampled":    {"00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-00", 0},
		"unknown flags":  {"00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-09", 9},
		"future version": {"cc-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01", 1},
		"future extra": {
			"cc-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01-what-the-future-will-be-like",
			1,
		},
		"surrounding space": {"  " + example + " ", 1},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, err := tracectx.Parse(c.in)
			if err != nil {
				t.Fatal(err)
			}
			if got.TraceID.String() != trace || got.SpanID.String() != span ||
				got.Flags != c.flags {
				t.Fatalf("got %+v", got)
			}
		})
	}
}

func TestParseRejects(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"empty":                "",
		"short":                "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-0",
		"zero trace id":        "00-00000000000000000000000000000000-b7ad6b7169203331-01",
		"zero parent id":       "00-0af7651916cd43dd8448eb211c80319c-0000000000000000-01",
		"version ff":           "ff-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01",
		"uppercase trace id":   "00-0AF7651916CD43DD8448EB211C80319C-b7ad6b7169203331-01",
		"uppercase parent id":  "00-0af7651916cd43dd8448eb211c80319c-B7AD6B7169203331-01",
		"uppercase flags":      "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-0A",
		"uppercase version":    "0A-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01",
		"non hex trace id":     "00-0af7651916cd43dd8448eb211c80319g-b7ad6b7169203331-01",
		"bad separator":        "00_0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01",
		"v00 trailing":         example + "-extra",
		"v00 trailing dash":    example + "-",
		"future no dash":       "cc-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01x",
		"future trailing char": "cc-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01.",
		"flags not hex":        "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-zz",
		"signs in hex":         "00-+af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01",
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got, err := tracectx.Parse(in); err == nil {
				t.Fatalf("accepted %q as %+v", in, got)
			}
		})
	}
}

func TestStringWritesVersion00(t *testing.T) {
	t.Parallel()
	c, err := tracectx.Parse("cc-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01-future")
	if err != nil {
		t.Fatal(err)
	}
	if got := c.String(); got != example {
		t.Fatalf("got %s", got)
	}
}

func TestNewRoot(t *testing.T) {
	t.Parallel()
	root := tracectx.NewRoot()
	if !root.Valid() || root.Flags != tracectx.FlagSampled {
		t.Fatalf("bad root %+v", root)
	}
	back, err := tracectx.Parse(root.String())
	if err != nil || back != root {
		t.Fatalf("round trip: %+v %v", back, err)
	}
	if other := tracectx.NewRoot(); other.TraceID == root.TraceID {
		t.Fatal("two roots share a trace id")
	}
}

func TestChild(t *testing.T) {
	t.Parallel()
	root := tracectx.NewRoot()
	root.TraceState = "vendor=x"
	child := tracectx.Child(root)
	if child.TraceID != root.TraceID || child.SpanID == root.SpanID || !child.Valid() {
		t.Fatalf("bad child %+v of %+v", child, root)
	}
	if child.Flags != root.Flags || child.TraceState != root.TraceState {
		t.Fatalf("flags or state lost: %+v", child)
	}
}

func TestChildClearsReservedFlags(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]byte{"09": 1, "fe": 0, "01": 1, "00": 0} {
		parent, err := tracectx.Parse("00-" + trace + "-" + span + "-" + in)
		if err != nil {
			t.Fatal(err)
		}
		child := tracectx.Child(parent)
		if child.Flags != want || !strings.HasSuffix(child.String(), fmt.Sprintf("-%02x", want)) {
			t.Errorf("flags %s: got %02x and %s, want %02x", in, child.Flags, child.String(), want)
		}
	}
}

func TestStringClearsReservedFlags(t *testing.T) {
	t.Parallel()
	c, err := tracectx.Parse("00-" + trace + "-" + span + "-09")
	if err != nil {
		t.Fatal(err)
	}
	if got := c.String(); got != example {
		t.Fatalf("got %s", got)
	}
}

const (
	other  = "A=1"
	otherB = "B=2"

	stateEnv = "TRACESTATE=vendor=x"
)

func TestFromEnviron(t *testing.T) {
	t.Parallel()
	environ := []string{other, "TRACEPARENT=" + example, stateEnv, otherB}
	c, ok := tracectx.FromEnviron(environ)
	if !ok || c.TraceState != "vendor=x" || c.TraceID.String() != trace {
		t.Fatalf("got %+v %v", c, ok)
	}
}

func TestToEnv(t *testing.T) {
	t.Parallel()
	environ := []string{other, "TRACEPARENT=" + example, stateEnv, otherB}
	c, _ := tracectx.FromEnviron(environ)
	child := tracectx.Child(c)
	out := child.ToEnv(environ)
	want := []string{other, otherB, "TRACEPARENT=" + child.String(), stateEnv}
	if !slices.Equal(out, want) {
		t.Fatalf("got %v want %v", out, want)
	}

	child.TraceState = ""
	out = child.ToEnv(environ)
	if len(out) != 3 || out[2] != "TRACEPARENT="+child.String() {
		t.Fatalf("tracestate not removed: %v", out)
	}
	if len(environ) != 4 || environ[1] != "TRACEPARENT="+example {
		t.Fatal("input environ was modified")
	}
}

func TestFromEnvironRejects(t *testing.T) {
	t.Parallel()
	cases := map[string][]string{
		"missing":   {other},
		"malformed": {"TRACEPARENT=nope"},
		"zero id":   {"TRACEPARENT=00-00000000000000000000000000000000-b7ad6b7169203331-01"},
	}
	for name, environ := range cases {
		if _, ok := tracectx.FromEnviron(environ); ok {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestFromEnvironDropsBadState(t *testing.T) {
	t.Parallel()
	long := make([]byte, 513)
	for i := range long {
		long[i] = 'a'
	}
	for _, state := range []string{"", "a=\x01", string(long)} {
		c, ok := tracectx.FromEnviron([]string{"TRACEPARENT=" + example, "TRACESTATE=" + state})
		if !ok || c.TraceState != "" {
			t.Errorf("state %q: got %+v %v", state, c, ok)
		}
	}
}

func TestFromEnv(t *testing.T) {
	t.Setenv("TRACEPARENT", example)
	t.Setenv("TRACESTATE", "k=v")
	c, ok := tracectx.FromEnv()
	if !ok || c.TraceState != "k=v" {
		t.Fatalf("got %+v %v", c, ok)
	}
	if err := os.Unsetenv("TRACEPARENT"); err != nil {
		t.Fatal(err)
	}
	if _, ok := tracectx.FromEnv(); ok {
		t.Fatal("accepted missing TRACEPARENT")
	}
}

func TestTraceStateGrammar(t *testing.T) {
	t.Parallel()
	members33 := strings.Join(distinctMembers(33), ",")
	dropped := map[string]string{
		"no equals":         "no-equals",
		"uppercase key":     "Vendor=x",
		"duplicate keys":    "a=1,b=2,a=3",
		"33 members":        members33,
		"empty key":         "=x",
		"empty value":       "a=",
		"value with equals": "a=b=c",
		"value tab":         "a=x\ty",
		"key starts digit":  "1a=x",
		"bad tenant key":    "@sys=x",
		"bad system key":    "t@1sys=x",
		"two at signs":      "a@b@c=x",
		"only separators":   " , ,\t",
		"too long":          "a=" + strings.Repeat("x", 511),
		"empties past 32":   "a=x" + strings.Repeat(",", 32),
		"33 slots empties":  distinctMembers(1)[0] + strings.Repeat(",,", 16) + ",",
	}
	for name, state := range dropped {
		t.Run("drop "+name, func(t *testing.T) {
			c, ok := tracectx.FromEnviron([]string{"TRACEPARENT=" + example, "TRACESTATE=" + state})
			if !ok || c.TraceState != "" {
				t.Fatalf("got %+v %v", c, ok)
			}
		})
	}
	kept := map[string]string{
		"single":        "a=x",
		"tab after":     "a=x,\tb=y",
		"spaces around": "a=x , b=y",
		"empty members": "a=x,,b=y,",
		"tenant key":    "tenant1@vendor=x",
		"key chars":     "a_b-c*d/e=x",
		"system chars":  "t@a_b-c*d/e=x",
		"digit tenant":  "1t@v=x",
		"value spaces":  "a=x y",
		"32 members":    strings.Join(distinctMembers(32), ","),
		"32 slots":      "a=x" + strings.Repeat(",", 31),
	}
	for name, state := range kept {
		t.Run("keep "+name, func(t *testing.T) {
			t.Parallel()
			c, ok := tracectx.FromEnviron([]string{"TRACEPARENT=" + example, "TRACESTATE=" + state})
			if !ok || c.TraceState != state {
				t.Fatalf("got %+v %v", c, ok)
			}
		})
	}
}

func TestTraceStateLimits(t *testing.T) {
	t.Parallel()
	pad := func(n int) string { return strings.Repeat("a", n) }
	value := func(n int) string { return strings.Repeat("v", n) }
	exactly512 := "k=" + value(256) + ",z=" + value(251)
	cases := map[string]struct {
		state string
		kept  bool
	}{
		"512 bytes":       {exactly512, true},
		"513 bytes":       {exactly512 + "v", false},
		"key 256":         {pad(256) + "=x", true},
		"key 257":         {pad(257) + "=x", false},
		"tenant id 241":   {pad(241) + "@s=x", true},
		"tenant id 242":   {pad(242) + "@s=x", false},
		"system id 14":    {"t@" + pad(14) + "=x", true},
		"system id 15":    {"t@" + pad(15) + "=x", false},
		"value 256 bytes": {"k=" + value(256), true},
		"value 257 bytes": {"k=" + value(257), false},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, ok := tracectx.FromEnviron(
				[]string{"TRACEPARENT=" + example, "TRACESTATE=" + c.state},
			)
			if !ok || (got.TraceState == c.state) != c.kept {
				t.Fatalf(
					"state of %d bytes: got %q, kept want %v",
					len(c.state),
					got.TraceState,
					c.kept,
				)
			}
		})
	}
}

func distinctMembers(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf("k%d=v", i)
	}
	return out
}
