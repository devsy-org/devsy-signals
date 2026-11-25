package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	baseRegistry = `classes: {internal: 1, config: 2}
codes:
  - {code: sample.error, class: internal, retryable: false, exit: 1, summary: Sample error}
`
	inventoryHeader = "classes: {internal: 1, config: 2}\ncodes:\n"
	sampleEntry     = "  - code: sample.error\n    class: internal\n    retryable: false\n    status: active\n"
	addedEntry      = "  - code: sample.added\n    class: internal\n    retryable: false\n    status: active\n"
)

func TestRunPreservesRegistryWithoutBaseInventory(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct{ current, want string }{
		"rename": {
			inventoryHeader + strings.Replace(sampleEntry, "sample.error", "sample.renamed", 1),
			`"sample.error" was removed`,
		},
		"remove": {"classes: {internal: 1, config: 2}\ncodes: []\n", `"sample.error" was removed`},
		"class": {
			inventoryHeader + strings.Replace(sampleEntry, "internal", "config", 1),
			"different class or retryable",
		},
		"retryable": {
			inventoryHeader + strings.Replace(sampleEntry, "false", "true", 1),
			"different class or retryable",
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir, base := setupRepo(t, baseRegistry, "")
			writeInventory(t, dir, tc.current)
			require.ErrorContains(t, run(t.Context(), dir, base), tc.want)
		})
	}
}

func TestRunIgnoresInventoryFormatting(t *testing.T) {
	t.Parallel()
	for name, firstAppend := range map[string]string{
		"quoted values":    strings.ReplaceAll(inventoryHeader+sampleEntry+addedEntry, "internal", "'internal'"),
		"no final newline": strings.TrimSuffix(inventoryHeader+sampleEntry+addedEntry, "\n"),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir, base := setupRepo(t, baseRegistry, inventoryHeader+sampleEntry)
			writeInventory(t, dir, firstAppend)
			require.NoError(t, run(t.Context(), dir, base), "first append")
			base = fixtureTree(t, dir)
			secondEntry := strings.Replace(addedEntry, "sample.added", "sample.second", 1)
			writeInventory(t, dir, strings.TrimSuffix(firstAppend, "\n")+"\n"+secondEntry)
			require.NoError(t, run(t.Context(), dir, base), "second append")
		})
	}
}

func TestRunAcceptsCRLFAppendAndRetirement(t *testing.T) {
	t.Parallel()
	dir, base := setupRepo(t, baseRegistry, crlf(inventoryHeader+sampleEntry))
	retired := strings.Replace(sampleEntry, "active", "retired", 1)
	writeInventory(t, dir, crlf(inventoryHeader+retired+addedEntry))
	require.NoError(t, run(t.Context(), dir, base))
}

func TestRunUsesInventoryBeforeRegistry(t *testing.T) {
	t.Parallel()
	retired := strings.Replace(sampleEntry, "active", "retired", 1)
	dir, base := setupRepo(t, baseRegistry, inventoryHeader+retired)
	writeInventory(t, dir, inventoryHeader+sampleEntry)
	require.ErrorContains(t, run(t.Context(), dir, base), "cannot be reactivated")
}

func TestRunAllowsRetirementFromRegistryBaseline(t *testing.T) {
	t.Parallel()
	dir, base := setupRepo(t, baseRegistry, "")
	retired := strings.Replace(sampleEntry, "active", "retired", 1)
	writeInventory(t, dir, inventoryHeader+retired+addedEntry)
	require.NoError(t, run(t.Context(), dir, base))
}

func TestRunSkipsOnlyMissingBaseline(t *testing.T) {
	t.Parallel()
	dir, base := setupRepo(t, "", "")
	require.NoError(t, run(t.Context(), dir, base))
	require.ErrorContains(t, run(t.Context(), dir, "no-such-base"), "git ls-tree")
	dir, base = setupRepo(t, baseRegistry, "")
	require.ErrorContains(t, run(t.Context(), dir, base), "read inventory")
}

func crlf(s string) string {
	return strings.ReplaceAll(s, "\n", "\r\n")
}

func setupRepo(t *testing.T, registry, inventory string) (string, string) {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "schema"), 0o750))
	fixtureGit(t, dir, "init", "-q")
	if registry != "" {
		require.NoError(t, os.WriteFile(filepath.Join(dir, registryPath), []byte(registry), 0o600))
	}
	if inventory != "" {
		writeInventory(t, dir, inventory)
	}
	return dir, fixtureTree(t, dir)
}

func writeInventory(t *testing.T, dir, inventory string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, inventoryPath), []byte(inventory), 0o600))
}

func fixtureTree(t *testing.T, dir string) string {
	t.Helper()
	fixtureGit(t, dir, "-c", "core.autocrlf=false", "add", ".")
	return fixtureGit(t, dir, "write-tree")
}

func fixtureGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	//nolint:gosec // Each test supplies a fixed Git command in its temporary repository.
	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "%s", out)
	return strings.TrimSpace(string(out))
}
