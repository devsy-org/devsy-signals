package registry_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/devsy-org/devsy-signals/tools/internal/registry"
)

const wantInteger = "integer"

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
		"new bare code": header + "  - {code: failure, class: internal, retryable: false, exit: 1, summary: s}\n",
		"three segments": header +
			"  - {code: a.b.c, class: internal, retryable: false, exit: 1, summary: s}\n",
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
	for _, code := range []string{"class.internal", "class.transient"} {
		t.Run(code, func(t *testing.T) {
			t.Parallel()
			entry := "  - {code: " + code + ", class: internal, retryable: false, exit: 1, summary: s}\n"
			_, err := registry.Parse([]byte(header + entry))
			assert.ErrorContains(t, err, "reserved")
		})
	}
}

func TestParseRejectsInvalidSummary(t *testing.T) {
	t.Parallel()
	const singleLine = "summary must be a single line"
	for name, tc := range map[string]struct {
		summary string
		want    string
	}{
		"blank":     {`"  \t  "`, "summary must not be blank"},
		"multiline": {`"first\nsecond"`, singleLine},
		"carriage":  {`"first\rsecond"`, singleLine},
		"unicode":   {`"first\u2028second"`, singleLine},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			entry := "  - {code: a.b, class: internal, retryable: false, exit: 1, summary: " + tc.summary + "}\n"
			_, err := registry.Parse([]byte(header + entry))
			assert.ErrorContains(t, err, tc.want)
		})
	}
}

func TestParseRejectsInvalidClassNames(t *testing.T) {
	t.Parallel()
	for _, class := range []string{"Bad", "bad-class", "bad__class", "bad_", "1bad"} {
		t.Run(class, func(t *testing.T) {
			t.Parallel()
			_, err := registry.Parse([]byte("classes: {" + class + ": 1}\ncodes: []\n"))
			assert.ErrorContains(t, err, "must be lowercase snake_case")
		})
	}
}

func TestParseRejectsControlAndBidiSummary(t *testing.T) {
	t.Parallel()
	const want = "summary must not contain control or bidirectional characters"
	for name, escapes := range map[string][]string{
		"C0":        {"\\u0000", "\\u0001", "\\u0009", "\\u001b", "\\u001f"},
		"DEL":       {"\\u007f"},
		"C1":        {"\\u0080", "\\u0090", "\\u009f"},
		"marks":     {"\\u061c", "\\u200e", "\\u200f"},
		"mongolian": {"\\u180e"},
		"embed":     {"\\u202a", "\\u202b", "\\u202c", "\\u202d", "\\u202e"},
		"isolates":  {"\\u2066", "\\u2067", "\\u2068", "\\u2069"},
	} {
		for _, escape := range escapes {
			t.Run(name+" "+escape, func(t *testing.T) {
				t.Parallel()
				entry := "  - {code: a.b, class: internal, retryable: false, exit: 1, summary: \"x" +
					escape + "y\"}\n"
				_, err := registry.Parse([]byte(header + entry))
				assert.ErrorContains(t, err, want)
			})
		}
	}
}

func TestParseAcceptsPlainSummaryPunctuation(t *testing.T) {
	t.Parallel()
	entry := "  - {code: a.b, class: internal, retryable: false, exit: 1, summary: \"Caf\\u00e9 (\\u2013) 'ok'.\"}\n"
	_, err := registry.Parse([]byte(header + entry))
	require.NoError(t, err)
}

const (
	usage    = "usage"
	canceled = "canceled"
)

func TestParseChecksClassExitRange(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		classes string
		want    string
	}{
		"zero":                  {"{" + usage + ": 0}", usage},
		"126":                   {"{" + usage + ": 126}", usage},
		"127":                   {"{" + usage + ": 127}", usage},
		"128":                   {"{" + usage + ": 128}", usage},
		"129":                   {"{" + usage + ": 129}", usage},
		"131":                   {"{" + usage + ": 131}", usage},
		"255":                   {"{" + usage + ": 255}", usage},
		"256":                   {"{" + usage + ": 256}", usage},
		"negative":              {"{" + usage + ": -1}", usage},
		"130 on another class":  {"{" + usage + ": 130}", usage},
		"canceled at 126":       {"{" + canceled + ": 126}", canceled},
		"canceled at 129":       {"{" + canceled + ": 129}", canceled},
		"canceled at 255":       {"{" + canceled + ": 255}", canceled},
		"130 beside canceled":   {"{" + canceled + ": 130, " + usage + ": 130}", usage},
		"130 and another class": {"{" + canceled + ": 1, " + usage + ": 130}", usage},
		"one":                   {"{" + usage + ": 1}", ""},
		"two":                   {"{" + usage + ": 2}", ""},
		"75":                    {"{" + usage + ": 75}", ""},
		"124":                   {"{" + usage + ": 124}", ""},
		"125":                   {"{" + usage + ": 125}", ""},
		"canceled at 130":       {"{" + canceled + ": 130}", ""},
		"canceled in range":     {"{" + canceled + ": 125}", ""},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := registry.Parse([]byte("classes: " + tc.classes + "\ncodes: []\n"))
			if tc.want == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, "class \""+tc.want+"\" exit")
		})
	}
}

func TestParseRequiresIntegerExits(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct{ value, want string }{
		"float exit":     {"125.9", wantInteger},
		"float 130 exit": {"130.9", wantInteger},
		"exponent exit":  {"1e2", wantInteger},
		"quoted exit":    {`"2"`, wantInteger},
		"bool exit":      {"True", wantInteger},
		"null exit":      {"Null", wantInteger},
		"tilde exit":     {"~", wantInteger},
		"hex":            {"0x7E", "must be between"},
		"octal 0o":       {"0o17", ""},
		"plain":          {"7", ""},
		"hex in range":   {"0x7D", ""},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := registry.Parse([]byte("classes: {usage: " + tc.value + "}\ncodes: []\n"))
			if tc.want == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tc.want)
		})
	}
}

func TestParseRequiresIntegerEntryExit(t *testing.T) {
	t.Parallel()
	for name, exit := range map[string]string{
		"float entry":  ", exit: 1.5",
		"quoted entry": `, exit: "1"`,
		"null entry":   ", exit: null",
		"absent":       "",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			entry := "  - {code: a.b, class: internal, retryable: false, summary: s" + exit + "}\n"
			_, err := registry.Parse([]byte(header + entry))
			require.ErrorContains(t, err, wantInteger)
		})
	}
}
