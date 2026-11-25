package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/devsy-org/devsy-signals/tools/internal/registry"
)

func TestDecideBase(t *testing.T) {
	t.Parallel()
	_, err := decideBase("", true)
	require.ErrorContains(t, err, "base sha is empty")
	for _, base := range []string{"", zeroSHA} {
		skip, err := decideBase(base, false)
		require.NoError(t, err)
		require.True(t, skip, "push base %q", base)
	}
	skip, err := decideBase(zeroSHA, true)
	require.NoError(t, err)
	require.True(t, skip)
	skip, err = decideBase("abc123", true)
	require.NoError(t, err)
	require.False(t, skip)
}

func TestRunExplainsOversizedBaseBlob(t *testing.T) {
	t.Parallel()
	dir, base := setupRepo(
		t,
		"",
		inventoryHeader+sampleEntry+"#"+strings.Repeat("x", registry.MaxFileBytes)+"\n",
	)
	writeInventory(t, dir, inventoryHeader+sampleEntry)
	err := run(t.Context(), dir, base)
	require.ErrorIs(t, err, registry.ErrTooLarge)
	require.ErrorContains(t, err, "a maintainer commit must also shrink the file below")
}
