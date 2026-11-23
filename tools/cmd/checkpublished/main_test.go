package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCheckAppendOnly(t *testing.T) {
	t.Parallel()
	const prefix = "codes:\n  - code: sample.error\n"
	const active = "    status: active\n"
	const retired = "    status: retired\n"
	const addition = "  - code: sample.new\n" + active
	for name, tc := range map[string]struct {
		base    string
		current string
		want    string
	}{
		"new inventory": {current: prefix + active},
		"unchanged":     {base: prefix + active, current: prefix + active},
		"append":        {base: prefix + active, current: prefix + active + addition},
		"retire":        {base: prefix + active, current: prefix + retired},
		"remove":        {base: prefix + active, current: prefix, want: ":3: existing line removed"},
		"rename": {
			base: prefix + active, current: strings.Replace(prefix, "sample.error", "sample.other", 1) + active,
			want: ":2: existing line changed",
		},
		"insert": {
			base: prefix + active, current: "codes:\n" + addition + "  - code: sample.error\n" + active,
			want: ":2: existing line changed",
		},
		"reactivate": {
			base: prefix + retired, current: prefix + active, want: ":3: existing line changed",
		},
		"strip newline": {
			base: prefix + active, current: prefix + strings.TrimSuffix(active, "\n"),
			want: ":3: existing line changed",
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			err := checkAppendOnly(tc.base, tc.current)
			if tc.want == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tc.want)
			}
		})
	}
}
