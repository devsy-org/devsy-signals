package attrkey_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
	"strconv"
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

func declaredKeys(t *testing.T) map[string]string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), "attrkey.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	keys := map[string]string{}
	ast.Inspect(file, func(n ast.Node) bool {
		spec, ok := n.(*ast.ValueSpec)
		if !ok {
			return true
		}
		for i, name := range spec.Names {
			lit, isLit := spec.Values[i].(*ast.BasicLit)
			if !isLit {
				t.Fatalf("%s is not a string literal", name.Name)
			}
			value, err := strconv.Unquote(lit.Value)
			if err != nil {
				t.Fatal(err)
			}
			keys[name.Name] = value
		}
		return false
	})
	return keys
}
