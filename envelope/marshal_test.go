package envelope_test

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/devsy-org/devsy-signals/envelope"
)

func TestMarshalJSONPreservesHTMLCharacters(t *testing.T) {
	t.Parallel()
	path := filepath.Join("..", "fixtures", "envelope", "valid", "log_html_message.json")
	e, err := decodeFile(t, path)
	if err != nil {
		t.Fatal(err)
	}
	e.Data = map[string]any{"output": "<>&"}
	e.Attrs = map[string]any{"output": "<>&"}
	wire := mustMarshalJSON(t, e)
	checkWireFragments(
		t,
		wire,
		`"msg":"a<b>&c"`,
		`"data":{"output":"<>&"}`,
		`"attrs":{"output":"<>&"}`,
	)
	if bytes.HasSuffix(wire, []byte{'\n'}) {
		t.Errorf("unexpected trailing newline in %q", wire)
	}
	back, err := envelope.DecodeStrict(wire)
	if err != nil {
		t.Fatal(err)
	}
	again := mustMarshalJSON(t, back)
	if !reflect.DeepEqual(back, e) || !bytes.Equal(again, wire) {
		t.Errorf("round trip changed the record or its encoding: %s", again)
	}
	checkUnescapedEncoder(t, e, wire)
	checkDefaultMarshalHTML(t, e, wire)
}

func checkUnescapedEncoder(t *testing.T, e envelope.Envelope, wire []byte) {
	t.Helper()
	var output bytes.Buffer
	enc := json.NewEncoder(&output)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(e); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(bytes.TrimSuffix(output.Bytes(), []byte{'\n'}), wire) {
		t.Errorf("encoder with HTML escaping disabled changed the wire bytes: %q", output.Bytes())
	}
}

func checkDefaultMarshalHTML(t *testing.T, e envelope.Envelope, wire []byte) {
	t.Helper()
	encoded, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	escape := strings.NewReplacer("<", `\u003c`, ">", `\u003e`, "&", `\u0026`)
	if string(encoded) != escape.Replace(string(wire)) {
		t.Errorf("expected json.Marshal to escape all HTML characters: %s", encoded)
	}
	back, err := envelope.Decode(encoded)
	if err != nil || !reflect.DeepEqual(back, e) {
		t.Errorf("escaped JSON did not decode to the same record: %v", err)
	}
}

func TestMarshalJSONPreservesEmptyMaps(t *testing.T) {
	t.Parallel()
	e := resultRecord(t)
	e.Attrs = map[string]any{}
	wire := mustMarshalJSON(t, e)
	checkWireFragments(t, wire, `"data":{}`, `"attrs":{}`)
	back, err := envelope.DecodeStrict(wire)
	if err != nil || !reflect.DeepEqual(back, e) {
		t.Errorf("empty maps did not round-trip: %v", err)
	}
}

func checkWireFragments(t *testing.T, wire []byte, fragments ...string) {
	t.Helper()
	for _, fragment := range fragments {
		if !bytes.Contains(wire, []byte(fragment)) {
			t.Errorf("missing literal %s in %s", fragment, wire)
		}
	}
}

func mustMarshalJSON(t *testing.T, e envelope.Envelope) []byte {
	t.Helper()
	wire, err := e.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	return wire
}
