package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRunRejectsInventoryHistoryChanges(t *testing.T) {
	t.Parallel()
	retired := strings.Replace(sampleEntry, "active", "retired", 1)
	for name, tc := range map[string]struct{ base, current, want string }{
		"reorder": {
			inventoryHeader + sampleEntry + addedEntry,
			inventoryHeader + addedEntry + sampleEntry,
			"changed position",
		},
		"insert": {
			inventoryHeader + sampleEntry,
			inventoryHeader + addedEntry + sampleEntry,
			"changed position",
		},
		"reactivate": {
			inventoryHeader + retired,
			inventoryHeader + sampleEntry,
			"cannot be reactivated",
		},
		"remove": {
			inventoryHeader + sampleEntry + addedEntry,
			inventoryHeader + sampleEntry,
			"was removed",
		},
		"rename": {
			inventoryHeader + sampleEntry,
			inventoryHeader + addedEntry,
			"was removed",
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir, base := setupRepo(t, "", tc.base)
			writeInventory(t, dir, tc.current)
			require.ErrorContains(t, run(t.Context(), dir, base), tc.want)
		})
	}
}
