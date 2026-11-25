package registry_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/devsy-org/devsy-signals/tools/internal/registry"
)

func doc(lines ...string) string { return strings.Join(lines, "\n") + "\n" }

func crlf(s string) string { return strings.ReplaceAll(s, "\n", "\r\n") }

// blockEntry is one codes entry in block style.
func blockEntry(retryable, exit string) string {
	return doc(
		"classes: {internal: 1}",
		"codes:",
		"  - code: a.b",
		"    class: internal",
		"    retryable: "+retryable,
		"    exit: "+exit,
		"    summary: s",
	)
}

// flowEntry is one codes entry in flow style with a summary before the exit.
func flowEntry(summary, retryable, exit string) string {
	return doc(
		"classes: {internal: 1}",
		"codes:",
		"  - {code: a.b, class: internal, retryable: "+retryable+", summary: "+summary+", exit: "+exit+"}",
	)
}

func TestParseRejectsBareTagOnExit(t *testing.T) {
	t.Parallel()
	for name, src := range map[string]string{
		"class flow":       doc("classes: {internal: ! 1}", "codes: []"),
		"class block":      doc("classes:", "  internal: ! 1", "codes: []"),
		"class anchor tag": doc("classes: {internal: &a ! 1}", "codes: []"),
		"class tag anchor": doc("classes: {internal: ! &a 1}", "codes: []"),
		"class next line":  doc("classes:", "  internal: !", "    1", "codes: []"),
		"class crlf":       crlf(doc("classes:", "  internal: ! 1", "codes: []")),
		"class bom":        "\ufeff" + doc("classes: {internal: ! 1}", "codes: []"),
		"entry flow":       flowEntry("s", "false", "! 1"),
		"entry anchor tag": flowEntry("s", "false", "&a ! 1"),
		"entry tag anchor": flowEntry("s", "false", "! &a 1"),
		"entry block":      blockEntry("false", "! 1"),
		"entry crlf":       crlf(blockEntry("false", "! 1")),
		"entry quoted":     flowEntry(`"wow !"`, "false", "! 1"),
		"entry unicode":    flowEntry("\"caf\u00e9 \u4e16\"", "false", "! 1"),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := registry.Parse([]byte(src))
			require.ErrorContains(t, err, "exit must be a plain integer")
			if strings.HasPrefix(name, "entry") {
				require.ErrorContains(t, err, `code "a.b"`)
			}
		})
	}
}

func TestParseRejectsBareTagOnRetryable(t *testing.T) {
	t.Parallel()
	for name, retryable := range map[string]string{
		"flow":       "! false",
		"anchor tag": "&a ! true",
		"tag anchor": "! &a true",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := registry.Parse([]byte(flowEntry("s", retryable, "1")))
			require.ErrorContains(t, err, "retryable must be true or false")
			require.ErrorContains(t, err, `code "a.b"`)
			require.ErrorContains(t, err, "line 3")
		})
	}
}

func TestParseKeepsBangInTextAndComments(t *testing.T) {
	t.Parallel()
	for name, src := range map[string]string{
		"comment line":    "# ! comment\n" + blockEntry("false", "1"),
		"comment after":   blockEntry("false", "1 # !"),
		"quoted summary":  flowEntry(`"wow !"`, "false", "1"),
		"plain summary":   flowEntry("wow !", "false", "1"),
		"bang then value": flowEntry(`"! 1"`, "true", "1"),
		"crlf plain":      crlf(blockEntry("true", "1 # !")),
		"bom plain":       "\ufeff" + blockEntry("true", "1"),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := registry.Parse([]byte(src))
			require.NoError(t, err)
		})
	}
}

func TestLoadInventoryRejectsBareTags(t *testing.T) {
	t.Parallel()
	inventory := func(classExit, retryable string) string {
		return doc(
			"classes: {internal: "+classExit+"}",
			"codes:",
			"  - code: a.b",
			"    class: internal",
			"    retryable: "+retryable,
			"    status: active",
		)
	}
	for name, tc := range map[string]struct{ src, want string }{
		"class exit": {inventory("! 1", "false"), "exit must be a plain integer"},
		"retryable":  {inventory("1", "! false"), "retryable must be true or false"},
		"retryable crlf": {
			crlf(inventory("1", "! false")),
			"retryable must be true or false",
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := registry.LoadInventory(strings.NewReader(tc.src))
			require.ErrorContains(t, err, tc.want)
		})
	}
	_, err := registry.LoadInventory(strings.NewReader("# ! 1\n" + inventory("1", "false")))
	require.NoError(t, err)
}
