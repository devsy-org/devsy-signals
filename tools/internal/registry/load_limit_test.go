package registry_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/devsy-org/devsy-signals/tools/internal/registry"
)

const (
	allocHelperEnv = "REGISTRY_ALLOC_HELPER"
	oversizeBytes  = 24 << 20
	allocSlack     = 8 << 20
)

// TestLoadBoundsAllocationOfOversizedFile re-runs the helper below alone in a
// child process, because allocation counts are process-wide and the other
// parallel tests would add noise.
func TestLoadBoundsAllocationOfOversizedFile(t *testing.T) {
	t.Parallel()
	//nolint:gosec // re-runs this test binary with a fixed helper name
	cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestAllocHelperLoad$")
	cmd.Env = append(os.Environ(), allocHelperEnv+"=1")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "%s", out)
}

func TestAllocHelperLoad(t *testing.T) {
	t.Parallel()
	if os.Getenv(allocHelperEnv) == "" {
		t.Skip("runs in a child process")
	}
	path := filepath.Join(t.TempDir(), "errors.yaml")
	require.NoError(t, os.WriteFile(path, make([]byte, oversizeBytes), 0o600))
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	_, err := registry.Load(path)
	runtime.ReadMemStats(&after)
	require.ErrorContains(t, err, path)
	require.ErrorContains(t, err, "too large")
	require.Less(t, after.TotalAlloc-before.TotalAlloc, uint64(allocSlack))
}
