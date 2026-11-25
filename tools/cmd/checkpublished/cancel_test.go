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

// standInGit writes a git script and returns its directory and the file the
// script touches when it is ready. The caller puts the directory on PATH.
func standInGit(t *testing.T, body string) (string, string) {
	t.Helper()
	bin := t.TempDir()
	ready := filepath.Join(bin, "ready")
	script := "#!/bin/sh\nREADY='" + ready + "'\n" + body
	git := filepath.Join(bin, "git")
	require.NoError(t, os.WriteFile(git, []byte(script), 0o600))
	require.NoError(t, os.Chmod(git, 0o700)) //nolint:gosec // the stand-in git must be executable
	return bin, ready
}

// cancelWhenReady cancels the context once the stand-in git has signalled, so
// the cancellation cannot land before git has started.
func cancelWhenReady(t *testing.T, ready string, cancel context.CancelFunc) {
	t.Helper()
	go func() {
		deadline := time.Now().Add(10 * time.Second)
		for time.Now().Before(deadline) {
			if _, err := os.Stat(ready); err == nil {
				cancel()
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		cancel()
	}()
}

// A cancellation that kills a running Git must surface as the context error,
// not as "signal: killed". The stand-in Git blocks until it is killed.
func TestGitBlobKeepsContextErrorWhenKilled(t *testing.T) {
	bin, ready := standInGit(t, "touch \"$READY\"\nexec sleep 30\n")
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	cancelWhenReady(t, ready, cancel)
	_, err := gitBlob(ctx, t.TempDir(), "HEAD:schema/errors.published.yaml")
	require.ErrorIs(t, err, context.Canceled)
}

// Git failed on its own before the cancellation, so its exit status must not
// be replaced by the context error. A background child keeps stdout open
// until after the cancellation.
func TestGitBlobKeepsGitFailureAfterCancel(t *testing.T) {
	bin, ready := standInGit(t, "(sleep 0.5; touch \"$READY\"; sleep 0.5) &\nexit 42\n")
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	cancelWhenReady(t, ready, cancel)
	_, err := gitBlob(ctx, t.TempDir(), "HEAD:schema/errors.published.yaml")
	require.ErrorContains(t, err, "exit status 42")
	require.NotErrorIs(t, err, context.Canceled)
}
