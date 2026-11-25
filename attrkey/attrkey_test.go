package attrkey_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// keyRE and maxKeyBytes match the attrs rules of the envelope schema.
var keyRE = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z0-9_]+)*$`)

const maxKeyBytes = 128

func TestKeysAreWellFormedAndUnique(t *testing.T) {
	t.Parallel()
	keys := declaredKeys(t)
	if len(keys) == 0 {
		t.Fatal("no keys found")
	}
	seen := map[string]string{}
	for name, key := range keys {
		if !keyRE.MatchString(key) || len(key) > maxKeyBytes {
			t.Errorf("%s = %q is not a valid attribute key", name, key)
		}
		if other, dup := seen[key]; dup {
			t.Errorf("%s and %s share the key %q", name, other, key)
		}
		seen[key] = name
	}
	for _, banned := range []string{"url.full", "time", "level", "msg", "source"} {
		if name, ok := seen[banned]; ok {
			t.Errorf("%s uses the forbidden key %q", name, banned)
		}
	}
}

// declaredKeys reads every constant in the package source, so a key added in
// any file is checked.
func declaredKeys(t *testing.T) map[string]string {
	t.Helper()
	paths, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	keys := map[string]string{}
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range file.Decls {
			if gen, ok := decl.(*ast.GenDecl); ok && gen.Tok == token.CONST {
				addConsts(t, gen, keys)
			}
		}
	}
	return keys
}

func addConsts(t *testing.T, gen *ast.GenDecl, keys map[string]string) {
	t.Helper()
	for _, s := range gen.Specs {
		spec, _ := s.(*ast.ValueSpec)
		if spec == nil || len(spec.Values) != len(spec.Names) {
			t.Fatalf("every key constant needs its own string literal at %v", s.Pos())
		}
		for i, name := range spec.Names {
			lit, isLit := spec.Values[i].(*ast.BasicLit)
			if !isLit || lit.Kind != token.STRING {
				t.Fatalf("%s is not a string literal", name.Name)
			}
			value, err := strconv.Unquote(lit.Value)
			if err != nil {
				t.Fatal(err)
			}
			keys[name.Name] = value
		}
	}
}
