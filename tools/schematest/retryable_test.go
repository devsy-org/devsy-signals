package schematest_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/devsy-org/devsy-signals/envelope"
)

func TestMissingRetryableRejectedOnWire(t *testing.T) {
	t.Parallel()
	raw, err := readFile(
		filepath.Join(root, "fixtures/envelope/invalid/error_missing_retryable.json"),
	)
	require.NoError(t, err)
	_, err = envelope.Decode(raw)
	require.NoError(t, err, "tolerant decoding does not preserve field presence")

	err = goVerdict(raw)
	require.ErrorIs(t, err, envelope.ErrStrict)
	require.ErrorContains(t, err, "retryable is required")
	require.ErrorContains(t, schemaVerdict(t, compile(t), raw), "retryable")
}
