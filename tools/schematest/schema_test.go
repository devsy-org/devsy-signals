package schematest_test

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/devsy-org/devsy-signals/envelope"
	"github.com/devsy-org/devsy-signals/tools/internal/registry"
)

const (
	root      = "../.."
	schemaURL = "https://github.com/devsy-org/devsy-signals/schema/envelope.schema.json"
	goOnlyDir = "go_only"
)

func compile(t *testing.T) *jsonschema.Schema {
	t.Helper()
	c := jsonschema.NewCompiler()
	for _, name := range []string{"envelope.schema.json", "audit-data.schema.json"} {
		raw, err := readFile(filepath.Join(root, "schema", name))
		require.NoError(t, err)
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
		require.NoError(t, err)
		require.NoError(
			t,
			c.AddResource("https://github.com/devsy-org/devsy-signals/schema/"+name, doc),
		)
	}
	s, err := c.Compile(schemaURL)
	require.NoError(t, err)
	return s
}

func readFile(path string) ([]byte, error) {
	return os.ReadFile(path) //nolint:gosec // fixture path from a fixed glob
}

func fixtureFiles(t *testing.T, repo, dir string) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(repo, "fixtures", "envelope", dir, "*.json"))
	require.NoError(t, err)
	if dir != goOnlyDir {
		require.NotEmpty(t, files)
	}
	return files
}

func schemaVerdict(t *testing.T, s *jsonschema.Schema, raw []byte) error {
	t.Helper()
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	require.NoError(t, err)
	return s.Validate(inst)
}

func goVerdict(raw []byte) error {
	e, err := envelope.DecodeStrict(raw)
	if err != nil {
		return err
	}
	return e.Validate()
}

func TestValidFixturesPassSchema(t *testing.T) {
	t.Parallel()
	s := compile(t)
	for _, path := range fixtureFiles(t, root, "valid") {
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()
			raw, err := readFile(path)
			require.NoError(t, err)
			assert.NoError(t, schemaVerdict(t, s, raw))
		})
	}
}

func TestInvalidFixturesFailSchema(t *testing.T) {
	t.Parallel()
	s := compile(t)
	for _, path := range fixtureFiles(t, root, "invalid") {
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()
			raw, err := readFile(path)
			require.NoError(t, err)
			assert.Error(t, schemaVerdict(t, s, raw))
		})
	}
}

func TestGoValidatorAgreesWithSchema(t *testing.T) {
	t.Parallel()
	s := compile(t)
	goOnly := loadGoOnly(t)
	for _, dir := range []string{"valid", "invalid", goOnlyDir} {
		for _, path := range fixtureFiles(t, root, dir) {
			name := dir + "/" + filepath.Base(path)
			reason := goOnly[name]
			delete(goOnly, name)
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				raw, err := readFile(path)
				require.NoError(t, err)
				assert.NoError(
					t,
					differentialVerdict(reason, schemaVerdict(t, s, raw), goVerdict(raw)),
				)
			})
		}
	}
	assert.Empty(t, goOnly, "missing Go-only fixtures")
}

func TestSchemaClassesMatchRegistry(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile(filepath.Join(root, "schema", "envelope.schema.json"))
	require.NoError(t, err)
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	require.NoError(t, err)
	enum := at[[]any](t, doc, "$defs", "error", "properties", "class", "enum")
	var got []string
	for _, c := range enum {
		class, ok := c.(string)
		require.True(t, ok, "class %v is not a string", c)
		got = append(got, class)
	}

	reg, err := registry.Load(filepath.Join(root, "schema", "errors.yaml"))
	require.NoError(t, err)
	var want []string
	for class := range reg.Classes {
		want = append(want, class)
	}
	slices.Sort(got)
	slices.Sort(want)
	assert.Equal(t, want, got)
}

// at walks nested JSON objects by key and returns the value as T.
func at[T any](t *testing.T, doc any, path ...string) T {
	t.Helper()
	cur := doc
	for _, key := range path {
		m, ok := cur.(map[string]any)
		require.True(t, ok, "%s: not an object", key)
		cur = m[key]
	}
	out, ok := cur.(T)
	require.True(t, ok, "%v: unexpected type %T", path, cur)
	return out
}
