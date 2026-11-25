package registry_test

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/devsy-org/devsy-signals/tools/internal/registry"
)

const (
	sampleCode  = "sample.error"
	sampleClass = "internal"
)

func TestPublishedInventory(t *testing.T) {
	t.Parallel()
	f, err := registry.Load("../../../schema/errors.yaml")
	require.NoError(t, err)

	inventory, err := os.Open("../../../schema/errors.published.yaml")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, inventory.Close()) })
	published, err := registry.LoadInventory(inventory)
	require.NoError(t, err)
	require.NoError(t, registry.CheckInventory(f, published))
}

func TestInventoryRejectsChanges(t *testing.T) {
	t.Parallel()
	const active = "  - {code: sample.error, class: internal, retryable: false, status: active}\n"
	for name, tc := range map[string]struct {
		code      registry.Entry
		inventory string
		want      string
	}{
		"rename": {
			registry.Entry{Code: "sample.renamed", Class: sampleClass},
			active,
			`registry code "sample.renamed" is missing from the inventory`,
		},
		"reuse retired": {
			registry.Entry{Code: sampleCode, Class: sampleClass},
			strings.Replace(active, "active", "retired", 1), `retired code "sample.error" is in the registry`,
		},
		"change class": {
			registry.Entry{Code: sampleCode, Class: "config"},
			active,
			`code "sample.error" has different class or retryable values`,
		},
		"change retryable": {
			registry.Entry{Code: sampleCode, Class: sampleClass, Retryable: true},
			active,
			`code "sample.error" has different class or retryable values`,
		},
		"remove active": {registry.Entry{}, active, `active code "sample.error" is missing from the registry`},
		"duplicate": {
			registry.Entry{Code: sampleCode, Class: sampleClass},
			active + active,
			`inventory code "sample.error" is listed twice`,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var f registry.File
			if tc.code.Code != "" {
				f.Codes = []registry.Entry{tc.code}
			}
			published, err := registry.LoadInventory(strings.NewReader("codes:\n" + tc.inventory))
			if err == nil {
				err = registry.CheckInventory(f, published)
			}
			require.ErrorContains(t, err, tc.want)
		})
	}
}

func TestLoadInventoryNullEntries(t *testing.T) {
	t.Parallel()
	const entry = "  - {code: sample.error, class: internal, retryable: false, status: active}\n"
	for name, tc := range map[string]struct{ src, want string }{
		"null before entry": {"codes:\n  - null\n" + entry, "inventory entry 0 is null"},
		"tilde entry":       {"codes:\n" + entry + "  - ~\n", "inventory entry 1 is null"},
		"empty item":        {"codes:\n  -\n" + entry, "inventory entry 0 is null"},
		"empty list":        {"codes: []\n", ""},
		"entry only":        {"codes:\n" + entry, ""},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := registry.LoadInventory(strings.NewReader(tc.src))
			if tc.want == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tc.want)
		})
	}
}

func TestRetryableMustBeYAMLBoolean(t *testing.T) {
	t.Parallel()
	for _, value := range []string{"off", "yes", "on", "no", "y", "n", "'true'", "1"} {
		t.Run(value, func(t *testing.T) {
			t.Parallel()
			inventory := "codes:\n  - {code: sample.error, class: internal, retryable: " + value +
				", status: active}\n"
			_, err := registry.LoadInventory(strings.NewReader(inventory))
			require.ErrorContains(t, err, "retryable must be true or false")

			src := "classes: {internal: 1}\ncodes:\n  - {code: sample.error, class: internal, retryable: " +
				value + ", exit: 1, summary: Sample}\n"
			_, err = registry.Parse([]byte(src))
			require.ErrorContains(t, err, "retryable must be true or false")
		})
	}
	for _, value := range []string{"true", "false"} {
		inventory := "codes:\n  - {code: sample.error, class: internal, retryable: " + value +
			", status: active}\n"
		_, err := registry.LoadInventory(strings.NewReader(inventory))
		require.NoError(t, err)
	}
}

func TestLoadInventoryRejectsMalformedCodes(t *testing.T) {
	t.Parallel()
	for _, code := range []string{"Bad.Code", "a.B", "a.", "a.b.c", "failure.x y", "class", ".a", "a b", "a.b\\u202e"} {
		t.Run(code, func(t *testing.T) {
			t.Parallel()
			src := "codes:\n  - {code: \"" + code + "\", class: internal, retryable: false, status: active}\n"
			_, err := registry.LoadInventory(strings.NewReader(src))
			require.ErrorContains(t, err, "must have the form domain.reason")
		})
	}
	for _, code := range []string{"unknown", "x1.y2", "a_b.c_d"} {
		src := "codes:\n  - {code: " + code + ", class: internal, retryable: false, status: active}\n"
		_, err := registry.LoadInventory(strings.NewReader(src))
		require.NoError(t, err, code)
	}
}
