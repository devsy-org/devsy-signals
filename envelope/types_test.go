package envelope_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/devsy-org/devsy-signals/envelope"
)

const srcCLI = "cli"

func statusWithEmptyData() envelope.Envelope {
	return envelope.Envelope{
		V:     1,
		Kind:  envelope.KindStatus,
		Type:  "a.b",
		ID:    envelope.NewID(),
		TS:    envelope.FormatTime(time.Now()),
		Src:   srcCLI,
		Level: envelope.LevelInfo,
		Data:  map[string]any{},
	}
}

func TestEmptyDataStaysOnTheWire(t *testing.T) {
	t.Parallel()
	out, err := json.Marshal(statusWithEmptyData())
	if err != nil || !strings.Contains(string(out), `"data":{}`) {
		t.Fatalf("got %s, %v", out, err)
	}
}

func TestSeverity(t *testing.T) {
	t.Parallel()
	want := map[envelope.Level]int{
		envelope.LevelTrace: 1, envelope.LevelDebug: 5, envelope.LevelInfo: 9,
		envelope.LevelWarn: 13, envelope.LevelError: 17,
	}
	for level, n := range want {
		if got := level.Severity(); got != n || !level.Known() {
			t.Errorf("%s = %d, want %d", level, got, n)
		}
	}
	if envelope.Level("fatal").Known() || envelope.Level("").Severity() != 0 {
		t.Error("unrecognized level is known")
	}
}

func TestKindKnown(t *testing.T) {
	t.Parallel()
	for _, kind := range []envelope.Kind{
		envelope.KindLog, envelope.KindStatus, envelope.KindResult, envelope.KindError, envelope.KindAudit,
	} {
		if !kind.Known() {
			t.Errorf("kind %q is not known", kind)
		}
	}
	if envelope.Kind("future").Known() || envelope.Kind("").Known() {
		t.Error("unrecognized kind is known")
	}
}

func TestActorKindKnown(t *testing.T) {
	t.Parallel()
	for _, kind := range []envelope.ActorKind{
		envelope.ActorUser, envelope.ActorAgent, envelope.ActorAdapter, envelope.ActorLocalUser, envelope.ActorSystem,
	} {
		if !kind.Known() {
			t.Errorf("actor kind %q is not known", kind)
		}
	}
	if envelope.ActorKind("visitor").Known() || envelope.ActorKind("").Known() {
		t.Error("unrecognized actor kind is known")
	}
}

func TestFormatTime(t *testing.T) {
	t.Parallel()
	loc := time.FixedZone("x", 3600)
	got := envelope.FormatTime(time.Date(2026, 11, 22, 11, 15, 3, 123456789, loc))
	if got != "2026-11-22T10:15:03.123Z" {
		t.Fatal(got)
	}
}
