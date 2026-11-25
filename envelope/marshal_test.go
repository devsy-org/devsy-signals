package envelope_test

import (
	"bytes"
	"path/filepath"
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
	wire := mustMarshalJSON(t, e)
	if !bytes.Contains(wire, []byte(`"msg":"a<b>&c"`)) || bytes.HasSuffix(wire, []byte{'\n'}) {
		t.Errorf("want literal message and no trailing newline; got %s", wire)
	}
	back, err := envelope.DecodeStrict(wire)
	if err != nil {
		t.Fatal(err)
	}
	again := mustMarshalJSON(t, back)
	if back.Msg != e.Msg || !bytes.Equal(again, wire) {
		t.Errorf("round trip changed the message or its encoding: %s", again)
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
