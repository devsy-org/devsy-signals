package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGitBlobKeepsContextErrorBeforeStart(t *testing.T) {
	t.Parallel()
	dir, base := setupRepo(t, "", inventoryHeader+sampleEntry)
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := gitBlob(canceled, dir, base+":"+inventoryPath)
	require.ErrorIs(t, err, context.Canceled)
	require.ErrorIs(t, run(canceled, dir, base), context.Canceled)
}

// A deadline that kills a running Git must surface as the context error, not
// as "signal: killed". The stand-in Git blocks until it is killed.
func TestGitBlobKeepsContextErrorWhenKilled(t *testing.T) {
	bin := t.TempDir()
	script := "#!/bin/sh\nexec sleep 30\n"
	git := filepath.Join(bin, "git")
	require.NoError(t, os.WriteFile(git, []byte(script), 0o600))
	require.NoError(t, os.Chmod(git, 0o700)) //nolint:gosec // the stand-in git must be executable
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
	defer cancel()
	_, err := gitBlob(ctx, t.TempDir(), "HEAD:schema/errors.published.yaml")
	require.ErrorIs(t, err, context.DeadlineExceeded)
}
