package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRunRejectsSymlinkedSchemaPaths(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		link   func(t *testing.T, dir string)
		atBase bool
	}{
		"directory at base":     {linkSchemaDir, true},
		"inventory at base":     {linkSchemaFile(inventoryPath), true},
		"registry at base":      {linkSchemaFile(registryPath), true},
		"directory in worktree": {linkSchemaDir, false},
		"inventory in worktree": {linkSchemaFile(inventoryPath), false},
		"registry in worktree":  {linkSchemaFile(registryPath), false},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir, base := setupRepo(t, baseRegistry, inventoryHeader+sampleEntry)
			tc.link(t, dir)
			if tc.atBase {
				base = fixtureTree(t, dir)
				renamed := strings.Replace(sampleEntry, "sample.error", "sample.renamed", 1)
				writeInventory(t, dir, inventoryHeader+renamed)
			}
			require.ErrorContains(t, run(t.Context(), dir, base), "symlink")
		})
	}
}

func linkSchemaDir(t *testing.T, dir string) {
	t.Helper()
	require.NoError(t, os.Rename(filepath.Join(dir, "schema"), filepath.Join(dir, "definitions")))
	require.NoError(t, os.Symlink("definitions", filepath.Join(dir, "schema")))
}

func linkSchemaFile(path string) func(t *testing.T, dir string) {
	return func(t *testing.T, dir string) {
		t.Helper()
		target := filepath.Join(dir, path)
		real := target + ".real"
		require.NoError(t, os.Rename(target, real))
		require.NoError(t, os.Symlink(filepath.Base(real), target))
	}
}
