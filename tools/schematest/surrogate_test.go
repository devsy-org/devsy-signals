package schematest_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/devsy-org/devsy-signals/envelope"
)

func logWithMsg(escaped string) []byte {
	return []byte(`{"v":1,"kind":"log","ts":"2026-02-03T10:15:03.123Z","src":"cli",` +
		`"level":"info","msg":"` + escaped + `"}`)
}

// The Go decoder rejects an unpaired surrogate escape. The schema validator
// parses the same text with its own JSON reader and accepts it, so the verdicts
// differ and the Go side is the stricter one.
func TestLoneSurrogateEscapeIsGoOnly(t *testing.T) {
	t.Parallel()
	s := compile(t)
	for name, msg := range map[string]string{
		"high":              `a\ud800b`,
		"low":               `a\udc00b`,
		"high at end":       `a\ud83d`,
		"high then ascii":   `\ud800\u0041`,
		"low then high":     `\ude00\ud83d`,
		"high high low":     `\ud83d\ud83d\ude00`,
		"uppercase hex":     `a\uD800b`,
		"pair after a lone": `\ud800\ud83d\ude00`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			raw := logWithMsg(msg)
			require.NoError(t, schemaVerdict(t, s, raw), "schema verdict")
			err := goVerdict(raw)
			require.Error(t, err, "Go verdict")
			require.ErrorIs(t, err, envelope.ErrText)
			assert.ErrorContains(t, err, "unpaired surrogate")
		})
	}
}

func TestSurrogatePairIsAcceptedByBoth(t *testing.T) {
	t.Parallel()
	s := compile(t)
	raw := logWithMsg(`\ud83d\ude00`)
	require.NoError(t, schemaVerdict(t, s, raw))
	require.NoError(t, goVerdict(raw))
	e, err := envelope.DecodeStrict(raw)
	require.NoError(t, err)
	assert.Equal(t, "\U0001F600", e.Msg)
}
