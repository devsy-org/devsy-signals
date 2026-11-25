package schematest_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/devsy-org/devsy-signals/envelope"
)

func TestDepthFixtureBoundary(t *testing.T) {
	t.Parallel()
	s := compile(t)
	ok, err := readFile(filepath.Join(root, "fixtures/envelope/valid/result_data_depth_64.json"))
	require.NoError(t, err)
	require.NoError(t, goVerdict(ok))
	require.NoError(t, schemaVerdict(t, s, ok))

	tooDeep, err := readFile(
		filepath.Join(root, "fixtures/envelope/invalid/result_data_depth_65.json"),
	)
	require.NoError(t, err)
	_, err = envelope.Decode(tooDeep)
	require.ErrorIs(t, err, envelope.ErrDepth)
	require.ErrorIs(t, goVerdict(tooDeep), envelope.ErrDepth)
	require.Error(t, schemaVerdict(t, s, tooDeep))
}
