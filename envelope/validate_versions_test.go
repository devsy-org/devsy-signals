package envelope_test

import (
	"strings"
	"testing"
)

func TestValidateVersionHashes(t *testing.T) {
	t.Parallel()
	valid := "sha256:" + strings.Repeat("a", 64)
	for name, tc := range map[string]struct {
		value string
		valid bool
	}{
		"omitted":        {"", true},
		"valid":          {valid, true},
		"uppercase hex":  {"sha256:" + strings.Repeat("A", 64), false},
		"short":          {valid[:len(valid)-1], false},
		"missing prefix": {strings.Repeat("a", 64), false},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			e := statusWithEmptyData()
			e.PolicyVersion = tc.value
			if err := e.Validate(); (err == nil) != tc.valid {
				t.Errorf("policyVersion %q: got %v, valid=%v", tc.value, err, tc.valid)
			}
			e.PolicyVersion, e.ConfigVersion = "", tc.value
			if err := e.Validate(); (err == nil) != tc.valid {
				t.Errorf("configVersion %q: got %v, valid=%v", tc.value, err, tc.valid)
			}
		})
	}
}
