package schematest_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func loadGoOnly(t *testing.T) map[string]string {
	t.Helper()
	raw, err := readFile("go_only.json")
	require.NoError(t, err)
	var cases map[string]string
	require.NoError(t, json.Unmarshal(raw, &cases))
	require.NotNil(t, cases)
	for fixture, reason := range cases {
		require.NotEmpty(t, strings.TrimSpace(reason), fixture)
		require.NotContains(t, reason, "\n", fixture)
		require.NotContains(t, reason, "\r", fixture)
	}
	return cases
}

func differentialVerdict(reason string, schemaErr, goErr error) error {
	if reason != "" {
		if schemaErr != nil {
			return fmt.Errorf(
				"schema unexpectedly rejected Go-only case (%s): %w",
				reason,
				schemaErr,
			)
		}
		if goErr == nil {
			return fmt.Errorf("Go unexpectedly accepted Go-only case (%s)", reason)
		}
	} else if (schemaErr == nil) != (goErr == nil) {
		return fmt.Errorf(
			"unexpected Go and schema disagreement: %w",
			errors.Join(schemaErr, goErr),
		)
	}
	return nil
}

func TestDifferentialVerdict(t *testing.T) {
	t.Parallel()
	const byteLength = "UTF-8 byte length"
	rejected := errors.New("rejected")
	for _, tc := range []struct {
		name   string
		reason string
		schema error
		goErr  error
		pass   bool
	}{
		{name: "both accept", pass: true},
		{name: "both reject", schema: rejected, goErr: rejected, pass: true},
		{name: "unlisted Go rejection", goErr: rejected},
		{name: "unlisted schema rejection", schema: rejected},
		{name: "listed Go rejection", reason: byteLength, goErr: rejected, pass: true},
		{name: "listed but Go accepts", reason: byteLength},
		{name: "listed but schema rejects", reason: byteLength, schema: rejected, goErr: rejected},
		{name: "listed but only schema rejects", reason: byteLength, schema: rejected},
	} {
		assert.Equal(
			t,
			tc.pass,
			differentialVerdict(tc.reason, tc.schema, tc.goErr) == nil,
			tc.name,
		)
	}
}
