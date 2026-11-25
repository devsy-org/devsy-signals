package gen_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/devsy-org/devsy-signals/tools/internal/gen"
	"github.com/devsy-org/devsy-signals/tools/internal/registry"
)

const (
	root          = "../../.."
	classInternal = "internal"
)

func TestCheckedInFilesAreCurrent(t *testing.T) {
	t.Parallel()
	f, err := registry.Load(filepath.Join(root, "schema", "errors.yaml"))
	require.NoError(t, err)
	goSrc, tsSrc, err := gen.Render(f)
	require.NoError(t, err)

	for path, want := range map[string][]byte{
		"codes/codes_gen.go": goSrc,
		"ts/src/codes.ts":    tsSrc,
	} {
		got, err := os.ReadFile(filepath.Join(root, path)) //nolint:gosec // fixed generated paths
		require.NoError(t, err)
		assert.Equal(t, string(want), string(got), "%s is stale, run go run ./cmd/gencodes", path)
		assert.Contains(t, string(got), "Code generated")
		assert.Contains(t, string(got), "DO NOT EDIT.")
	}
}

func TestRenderRejectsConstantCollision(t *testing.T) {
	t.Parallel()
	f := registry.File{
		Classes: map[string]int{classInternal: 1},
		Codes: []registry.Entry{
			{Code: "a.b_c", Class: classInternal, Exit: 1, Summary: "s"},
			{Code: "a_b.c", Class: classInternal, Exit: 1, Summary: "s"},
		},
	}
	_, _, err := gen.Render(f)
	assert.ErrorContains(t, err, "both map to constant")
}

func TestRenderNames(t *testing.T) {
	t.Parallel()
	f := registry.File{
		Classes: map[string]int{"transient": 75},
		Codes: []registry.Entry{
			{Code: "ssh.auth_failed", Class: "transient", Exit: 75, Summary: "s."},
		},
	}
	goSrc, _, err := gen.Render(f)
	require.NoError(t, err)
	assert.Contains(t, string(goSrc), "SSHAuthFailed = \"ssh.auth_failed\"")
	assert.Contains(t, string(goSrc), "ClassTransient Class = \"transient\"")
}

func TestRenderQuotesTypeScriptStringsAsJSON(t *testing.T) {
	t.Parallel()
	f := registry.File{
		Classes: map[string]int{classInternal: 1},
		Codes: []registry.Entry{
			{Code: "a.b", Class: classInternal, Exit: 1, Summary: "A \"quote\", \\ and \a <tag>."},
		},
	}
	goSrc, tsSrc, err := gen.Render(f)
	require.NoError(t, err)
	assert.Contains(t, string(goSrc), `Summary: "A \"quote\", \\ and \a <tag>."`)
	assert.Contains(t, string(tsSrc), `summary: "A \"quote\", \\ and \u0007 \u003ctag\u003e."`)
}
