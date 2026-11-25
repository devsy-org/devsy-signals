package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	allocHelperEnv    = "CHECKPUBLISHED_ALLOC_HELPER"
	oversizeInventory = 24 << 20
	allocSlack        = 8 << 20
)

func bigInventory() string {
	return inventoryHeader + sampleEntry + "#" + strings.Repeat("x", oversizeInventory) + "\n"
}

// runHelper re-runs one helper test alone in a child process, because
// allocation counts are process-wide and the other parallel tests add noise.
func runHelper(t *testing.T, name string) {
	t.Helper()
	//nolint:gosec // re-runs this test binary with a fixed helper name
	cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^"+name+"$")
	cmd.Env = append(os.Environ(), allocHelperEnv+"=1")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "%s", out)
}

func measureRun(t *testing.T, dir, base string) (uint64, error) {
	t.Helper()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	err := run(t.Context(), dir, base)
	runtime.ReadMemStats(&after)
	return after.TotalAlloc - before.TotalAlloc, err
}

func TestRunBoundsOversizedBaseBlob(t *testing.T) {
	t.Parallel()
	runHelper(t, "TestAllocHelperBlob")
}

func TestRunBoundsOversizedCurrentFile(t *testing.T) {
	t.Parallel()
	runHelper(t, "TestAllocHelperCurrent")
}

func TestAllocHelperBlob(t *testing.T) {
	t.Parallel()
	if os.Getenv(allocHelperEnv) == "" {
		t.Skip("runs in a child process")
	}
	dir, base := setupRepo(t, "", bigInventory())
	writeInventory(t, dir, inventoryHeader+sampleEntry)
	used, err := measureRun(t, dir, base)
	require.ErrorContains(t, err, "too large")
	require.ErrorContains(t, err, inventoryPath)
	require.Less(t, used, uint64(allocSlack))
}

func TestAllocHelperCurrent(t *testing.T) {
	t.Parallel()
	if os.Getenv(allocHelperEnv) == "" {
		t.Skip("runs in a child process")
	}
	dir, base := setupRepo(t, "", inventoryHeader+sampleEntry)
	require.NoError(
		t,
		os.WriteFile(filepath.Join(dir, inventoryPath), []byte(bigInventory()), 0o600),
	)
	used, err := measureRun(t, dir, base)
	require.ErrorContains(t, err, "too large")
	require.ErrorContains(t, err, inventoryPath)
	require.Less(t, used, uint64(allocSlack))
}
