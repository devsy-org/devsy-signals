package registry_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/devsy-org/devsy-signals/tools/internal/registry"
)

const plainExitMsg = "exit must be a plain integer without a tag, anchor or alias"

func TestParseRejectsDecoratedClassExit(t *testing.T) {
	t.Parallel()
	for name, src := range map[string]string{
		"int tag":      "classes: {internal: !!int 1}\ncodes: []\n",
		"str tag":      "classes: {internal: !!str 1}\ncodes: []\n",
		"class custom": "classes: {internal: !custom 1}\ncodes: []\n",
		"class anchor": "classes: {internal: &a 1}\ncodes: []\n",
		"class alias":  "classes: {internal: &a 1, config: *a}\ncodes: []\n",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := registry.Parse([]byte(src))
			require.ErrorContains(t, err, plainExitMsg)
			assert.ErrorContains(t, err, "class ")
		})
	}
}

func TestParseRejectsDecoratedEntryExit(t *testing.T) {
	t.Parallel()
	const head = "classes: {internal: 1}\ncodes:\n"
	for name, src := range map[string]string{
		"int tag":      head + "  - {code: a.b, class: internal, retryable: false, exit: !!int 1, summary: s}\n",
		"str tag":      head + "  - {code: a.b, class: internal, retryable: false, exit: !!str 1, summary: s}\n",
		"custom tag":   head + "  - {code: a.b, class: internal, retryable: false, exit: !custom 1, summary: s}\n",
		"entry anchor": head + "  - {code: a.b, class: internal, retryable: false, exit: &a 1, summary: s}\n",
		"entry alias": "classes: {internal: &a 1}\ncodes:\n" +
			"  - {code: a.b, class: internal, retryable: false, exit: *a, summary: s}\n",
		"quoted": head + "  - {code: a.b, class: internal, retryable: false, exit: '1', summary: s}\n",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := registry.Parse([]byte(src))
			require.ErrorContains(t, err, wantInteger)
			if name != "quoted" && name != "entry alias" {
				assert.ErrorContains(t, err, "a.b")
			}
		})
	}
}

func TestParseRejectsDecoratedRetryable(t *testing.T) {
	t.Parallel()
	for name, value := range map[string]string{
		"bool tag":   "!!bool false",
		"custom tag": "!custom false",
		"anchor":     "&a false",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			entry := "  - {code: a.b, class: internal, retryable: " + value + ", exit: 1, summary: s}\n"
			_, err := registry.Parse([]byte(header + entry))
			require.ErrorContains(t, err, "retryable must be true or false")
		})
	}
}

func TestParsePlainExitStillPasses(t *testing.T) {
	t.Parallel()
	f, err := registry.Parse([]byte("classes: {internal: 1}\ncodes:\n" +
		"  - {code: a.b, class: internal, retryable: true, exit: 1, summary: s}\n"))
	require.NoError(t, err)
	assert.Equal(t, 1, f.Codes[0].Exit)
}
