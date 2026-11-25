package registry_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDuplicateKeysAreReportedOncePerMapping(t *testing.T) {
	t.Parallel()
	for lname, load := range loaders() {
		t.Run(lname, func(t *testing.T) {
			t.Parallel()
			src := "classes:\n" + strings.Repeat("  a: 1\n", 3000) + "codes: []\n"
			err := load([]byte(src))
			require.ErrorContains(t, err, "defined twice")
			require.Less(t, len(err.Error()), 1<<16)
		})
	}
}
