package registry_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/devsy-org/devsy-signals/tools/internal/registry"
)

const header = "classes: {internal: 1, transient: 75}\ncodes:\n"

func TestLoadSeedRegistry(t *testing.T) {
	t.Parallel()
	f, err := registry.Load("../../../schema/errors.yaml")
	require.NoError(t, err)
	assert.Equal(t, map[string]int{
		"internal": 1, "usage": 2, "config": 2, "environment": 3,
		"transient": 75, "recoverable_build": 79, "timeout": 124, "canceled": 130,
	}, f.Classes)

	have := map[string]bool{}
	for _, e := range f.Codes {
		have[e.Code] = true
	}
	for _, want := range []string{
		"unknown", "internal.panic", "canceled", "timeout", "input.required", "config.invalid",
		"api.rate_limited", "build.failed_recoverable", "docker.daemon_unreachable",
		"secret.unlock_required", "ssh.auth_failed", "provider.auth_failed", "provider.unreachable",
		"runtime.unknown", "runtime.canceled", "runtime.failure", "runtime.incompatible_version",
	} {
		assert.True(t, have[want], want)
	}
}

func TestParseRejects(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"bad format":    header + "  - {code: Bad.Code, class: internal, retryable: false, exit: 1, summary: s}\n",
		"uppercase":     header + "  - {code: a.B, class: internal, retryable: false, exit: 1, summary: s}\n",
		"empty segment": header + "  - {code: a., class: internal, retryable: false, exit: 1, summary: s}\n",
		"unknown class": header + "  - {code: a.b, class: nope, retryable: false, exit: 1, summary: s}\n",
		"wrong exit":    header + "  - {code: a.b, class: internal, retryable: false, exit: 2, summary: s}\n",
		"no summary":    header + "  - {code: a.b, class: internal, retryable: false, exit: 1}\n",
		"duplicate": header +
			"  - {code: a.b, class: internal, retryable: false, exit: 1, summary: s}\n" +
			"  - {code: a.b, class: internal, retryable: false, exit: 1, summary: s}\n",
		"unknown field": header + "  - {code: a.b, class: internal, retryable: false, exit: 1, summary: s, x: 1}\n",
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := registry.Parse([]byte(src))
			assert.Error(t, err)
		})
	}
}

func TestParseAcceptsBareAndDotted(t *testing.T) {
	t.Parallel()
	_, err := registry.Parse([]byte(header +
		"  - {code: unknown, class: internal, retryable: false, exit: 1, summary: s}\n" +
		"  - {code: a_b.c_d, class: transient, retryable: true, exit: 75, summary: s}\n"))
	require.NoError(t, err)
}

func TestParseRejectsMissingRetryable(t *testing.T) {
	t.Parallel()
	for name, entry := range map[string]string{
		"absent": "  - {code: a.b, class: internal, exit: 1, summary: s}\n",
		"null":   "  - {code: a.b, class: internal, retryable: null, exit: 1, summary: s}\n",
		"tilde":  "  - {code: a.b, class: internal, retryable: ~, exit: 1, summary: s}\n",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := registry.Parse([]byte(header + entry))
			assert.ErrorContains(t, err, "retryable")
		})
	}
}

func TestParseRejectsSecondDocument(t *testing.T) {
	t.Parallel()
	one := header + "  - {code: a.b, class: internal, retryable: false, exit: 1, summary: s}\n"
	_, err := registry.Parse([]byte(one + "---\n" + one))
	assert.ErrorContains(t, err, "one YAML document")
}

func TestParseRejectsReservedNames(t *testing.T) {
	t.Parallel()
	for _, code := range []string{"lookup", "all", "class", "info", "class_internal", "class.transient"} {
		t.Run(code, func(t *testing.T) {
			t.Parallel()
			entry := "  - {code: " + code + ", class: internal, retryable: false, exit: 1, summary: s}\n"
			_, err := registry.Parse([]byte(header + entry))
			assert.ErrorContains(t, err, "reserved")
		})
	}
}
