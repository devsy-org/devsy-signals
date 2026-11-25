package registry_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/devsy-org/devsy-signals/tools/internal/registry"
)

type runeRange struct{ lo, hi rune }

func TestDefaultIgnorable(t *testing.T) {
	t.Parallel()
	ignorable := []runeRange{
		{0x00ad, 0x00ad},
		{0x034f, 0x034f},
		{0x061c, 0x061c},
		{0x115f, 0x1160},
		{0x17b4, 0x17b5},
		{0x180b, 0x180f},
		{0x200b, 0x200f},
		{0x202a, 0x202e},
		{0x2060, 0x206f},
		{0x3164, 0x3164},
		{0xfe00, 0xfe0f},
		{0xfeff, 0xfeff},
		{0xffa0, 0xffa0},
		{0xfff0, 0xfff8},
		{0x1bca0, 0x1bca3},
		{0x1d173, 0x1d17a},
		{0xe0000, 0xe0fff},
	}
	plain := []runeRange{
		{'A', 'Z'},
		{'a', 'z'},
		{'0', '9'},
		{' ', '~'},
		{0x00a0, 0x00a0},
		{0x00e9, 0x00e9},
		{0x0301, 0x0301},
		{0x0600, 0x0605},
		{0x06dd, 0x06dd},
		{0x070f, 0x070f},
		{0x0890, 0x0891},
		{0x08e2, 0x08e2},
		{0x2013, 0x2013},
		{0x2028, 0x2029},
		{0x3000, 0x3000},
		{0x4e16, 0x4e16},
		{0xac00, 0xac00},
		{0xd7a3, 0xd7a3},
		{0xfff9, 0xfffb},
		{0x110bd, 0x110bd},
		{0x110cd, 0x110cd},
		{0x13430, 0x13440},
		{0x1f600, 0x1f600},
	}
	for _, rr := range ignorable {
		for r := rr.lo; r <= rr.hi; r++ {
			assert.True(t, registry.DefaultIgnorable(r), "U+%04X", r)
		}
	}
	for _, rr := range plain {
		for r := rr.lo; r <= rr.hi; r++ {
			assert.False(t, registry.DefaultIgnorable(r), "U+%04X", r)
		}
	}
}

func TestParseRejectsInvisibleSummary(t *testing.T) {
	t.Parallel()
	for _, r := range []rune{
		0x00ad, 0x034f, 0x061c, 0x115f, 0x1160, 0x17b4, 0x180b, 0x200b, 0x200c, 0x200d,
		0x2060, 0x2065, 0x3164, 0xfe0f, 0xfeff, 0xffa0, 0x1bca0, 0x1d173, 0xe0001, 0xe0100,
	} {
		t.Run(fmt.Sprintf("U+%04X", r), func(t *testing.T) {
			t.Parallel()
			entry := "  - {code: a.b, class: internal, retryable: false, exit: 1, summary: \"x" +
				string(r) + "y\"}\n"
			_, err := registry.Parse([]byte(header + entry))
			require.ErrorContains(t, err, "summary must be plain text")
		})
	}
}

func TestParseAcceptsPlainTextSummary(t *testing.T) {
	t.Parallel()
	for _, summary := range []string{
		"Plain ASCII.", "Caf\u00e9 e\u0301", "\u4e16\u754c", "\ud55c\uae00", "\U0001F600 ok",
	} {
		entry := "  - {code: a.b, class: internal, retryable: false, exit: 1, summary: \"" + summary + "\"}\n"
		_, err := registry.Parse([]byte(header + entry))
		require.NoError(t, err, summary)
	}
}
