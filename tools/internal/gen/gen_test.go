package gen_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
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
	assert.Contains(t, string(goSrc), "SSHAuthFailed Code = \"ssh.auth_failed\"")
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

func renderedNames(t *testing.T, f registry.File) []string {
	t.Helper()
	goSrc, _, err := gen.Render(f)
	require.NoError(t, err)
	parsed, err := parser.ParseFile(token.NewFileSet(), "codes_gen.go", goSrc, 0)
	require.NoError(t, err)
	return topLevelNames(parsed)
}

// An empty registry leaves no generated constants, so every exported name the
// template declares must be one of the fixed reserved names, and the reverse.
func TestReservedNamesMatchTemplateDeclarations(t *testing.T) {
	t.Parallel()
	declared := renderedNames(t, registry.File{})
	reserved := registry.GeneratedNames()
	for _, name := range declared {
		if ast.IsExported(name) {
			assert.Contains(t, reserved, name, "generated %s is not reserved", name)
		}
	}
	for _, name := range reserved {
		assert.Contains(t, declared, name, "reserved %s is not declared", name)
	}
}

func TestRealRegistryNamesAreReservedOrGenerated(t *testing.T) {
	t.Parallel()
	f, err := registry.Load(filepath.Join(root, "schema", "errors.yaml"))
	require.NoError(t, err)
	allowed := registry.GeneratedNames()
	for class := range f.Classes {
		allowed = append(allowed, "Class"+registry.ConstName(class))
	}
	for _, e := range f.Codes {
		allowed = append(allowed, registry.ConstName(e.Code))
	}
	for _, name := range renderedNames(t, f) {
		if ast.IsExported(name) {
			assert.True(t, slices.Contains(allowed, name), "unexpected generated name %s", name)
		}
	}
}

func topLevelNames(file *ast.File) []string {
	var names []string
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Recv == nil {
				names = append(names, d.Name.Name)
			}
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				names = append(names, specNames(spec)...)
			}
		}
	}
	return names
}

func specNames(spec ast.Spec) []string {
	switch s := spec.(type) {
	case *ast.TypeSpec:
		return []string{s.Name.Name}
	case *ast.ValueSpec:
		names := make([]string, 0, len(s.Names))
		for _, n := range s.Names {
			names = append(names, n.Name)
		}
		return names
	}
	return nil
}

func TestRenderAcceptsFormatCharactersTheRegistryAllows(t *testing.T) {
	t.Parallel()
	const src = "classes: {internal: 1}\ncodes:\n" +
		"  - {code: a.b, class: internal, retryable: false, exit: 1, " +
		"summary: \"soft\\u00adhyphen joiner\\u200d word\\u2060joiner\"}\n"
	f, err := registry.Parse([]byte(src))
	require.NoError(t, err)
	goSrc, tsSrc, err := gen.Render(f)
	require.NoError(t, err)
	_, err = parser.ParseFile(token.NewFileSet(), "codes_gen.go", goSrc, 0)
	require.NoError(t, err)
	assert.NotEmpty(t, tsSrc)
}
