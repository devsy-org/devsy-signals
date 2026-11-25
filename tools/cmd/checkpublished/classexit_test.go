package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func classed(classes string) string {
	return "classes: " + classes + "\ncodes:\n" + sampleEntry
}

func TestRunFreezesClassExits(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct{ current, want string }{
		"changed exit":  {classed("{internal: 9, config: 2}"), `class "internal" changed exit from 1 to 9`},
		"removed class": {classed("{internal: 1}"), `class "config" was removed`},
		"added class":   {classed("{internal: 1, config: 2, usage: 3}"), ""},
		"unchanged":     {classed("{config: 2, internal: 1}"), ""},
	} {
		for _, fromRegistry := range []bool{false, true} {
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				inventory := inventoryHeader + sampleEntry
				if fromRegistry {
					inventory = ""
				}
				dir, base := setupRepo(t, baseRegistry, inventory)
				writeInventory(t, dir, tc.current)
				err := run(t.Context(), dir, base)
				if tc.want == "" {
					require.NoError(t, err)
					return
				}
				require.ErrorContains(t, err, tc.want)
			})
		}
	}
}

func TestRunOldFormatBase(t *testing.T) {
	t.Parallel()
	dir, base := setupRepo(t, "", "codes:\n"+sampleEntry)
	writeInventory(t, dir, classed("{internal: 1, config: 2}"))
	require.NoError(t, run(t.Context(), dir, base), "adding class exits to an old inventory")

	writeInventory(t, dir, "codes:\n"+sampleEntry)
	require.ErrorContains(t, run(t.Context(), dir, base), "must record class exits")
}
