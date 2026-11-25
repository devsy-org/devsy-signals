package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func writeRegistry(t *testing.T, dir, registry string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, registryPath), []byte(registry), 0o600))
}

const baseClasses = "{internal: 1, config: 2}"

func registryOf(classes string, names ...string) string {
	var b strings.Builder
	b.WriteString("classes: " + classes + "\ncodes:\n")
	for _, name := range names {
		b.WriteString(
			"  - {code: " + name + ", class: internal, retryable: false, exit: 1, summary: Sample}\n",
		)
	}
	return b.String()
}

func TestRunChecksCurrentRegistryAgainstInventory(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct{ registry, want string }{
		"class exit changed": {
			strings.NewReplacer("internal: 1", "internal: 2", "exit: 1", "exit: 2").Replace(baseRegistry),
			`class "internal" has exit 2 in the registry but 1 in the inventory`,
		},
		"class removed": {
			strings.Replace(baseRegistry, ", config: 2", "", 1),
			`inventory class "config" is missing from the registry`,
		},
		"code class changed": {
			strings.NewReplacer("class: internal", "class: config", "exit: 1", "exit: 2").Replace(baseRegistry),
			"has different class or retryable values",
		},
		"code removed": {"classes: {internal: 1, config: 2}\ncodes: []\n", "missing from the registry"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir, base := setupRepo(t, baseRegistry, inventoryHeader+sampleEntry)
			writeRegistry(t, dir, tc.registry)
			require.ErrorContains(t, run(t.Context(), dir, base), tc.want)
		})
	}
}

func TestRunRequiresCurrentRegistry(t *testing.T) {
	t.Parallel()
	dir, base := setupRepo(t, baseRegistry, inventoryHeader+sampleEntry)
	require.NoError(t, os.Remove(filepath.Join(dir, registryPath)))
	require.ErrorContains(t, run(t.Context(), dir, base), "read registry")
}
