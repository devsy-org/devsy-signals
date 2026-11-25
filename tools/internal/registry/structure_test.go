package registry_test

import (
	"bytes"
	"encoding/binary"
	"os"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/stretchr/testify/require"

	"github.com/devsy-org/devsy-signals/tools/internal/registry"
)

const (
	notAllowed   = "not allowed in the registry"
	maxFileBytes = 1 << 20
	plainKeys    = "mapping keys must be plain strings"
)

type loader func([]byte) error

func loaders() map[string]loader {
	return map[string]loader{
		"parse": func(b []byte) error {
			_, err := registry.Parse(b)
			return err
		},
		"inventory": func(b []byte) error {
			_, err := registry.LoadInventory(bytes.NewReader(b))
			return err
		},
	}
}

func validFor(name string) string {
	if name == "parse" {
		return blockEntry("false", "1")
	}
	return publishedDoc("{internal: 1}", "false")
}

func publishedDoc(classes, retryable string) string {
	return doc(
		"classes: "+classes,
		"codes:",
		"  - code: a.b",
		"    class: internal",
		"    retryable: "+retryable,
		"    status: active",
	)
}

func utf16Bytes(s string, bigEndian, bom bool) []byte {
	var order binary.AppendByteOrder = binary.LittleEndian
	if bigEndian {
		order = binary.BigEndian
	}
	units := utf16.Encode([]rune(s))
	if bom {
		units = append([]uint16{0xfeff}, units...)
	}
	out := make([]byte, 0, 2*len(units))
	for _, u := range units {
		out = order.AppendUint16(out, u)
	}
	return out
}

func utf32Bytes(s string, bigEndian, bom bool) []byte {
	var order binary.AppendByteOrder = binary.LittleEndian
	if bigEndian {
		order = binary.BigEndian
	}
	runes := []rune(s)
	if bom {
		runes = append([]rune{0xfeff}, runes...)
	}
	out := make([]byte, 0, 4*len(runes))
	for _, r := range runes {
		if r < 0 {
			continue
		}
		out = order.AppendUint32(out, uint32(r))
	}
	return out
}

func TestRejectsOtherEncodings(t *testing.T) {
	t.Parallel()
	for lname, load := range loaders() {
		src := validFor(lname)
		for name, data := range map[string][]byte{
			"utf16le bom":    utf16Bytes(src, false, true),
			"utf16be bom":    utf16Bytes(src, true, true),
			"utf16le no bom": utf16Bytes(src, false, false),
			"utf16be no bom": utf16Bytes(src, true, false),
			"utf32le bom":    utf32Bytes(src, false, true),
			"utf32be bom":    utf32Bytes(src, true, true),
			"utf32le no bom": utf32Bytes(src, false, false),
			"utf32be no bom": utf32Bytes(src, true, false),
			"invalid utf8":   []byte(src + "# \xff\n"),
			"nul byte":       []byte(src + "# \x00\n"),
			"utf16 bare tag": utf16Bytes(strings.Replace(src, "1", "! 1", 1), false, true),
		} {
			t.Run(lname+" "+name, func(t *testing.T) {
				t.Parallel()
				require.ErrorContains(t, load(data), "UTF-8")
			})
		}
	}
}

func TestAcceptsUTF8ByteOrderMark(t *testing.T) {
	t.Parallel()
	for lname, load := range loaders() {
		t.Run(lname, func(t *testing.T) {
			t.Parallel()
			require.NoError(t, load([]byte("\xef\xbb\xbf"+validFor(lname))))
		})
	}
}

func TestLimitsFileSize(t *testing.T) {
	t.Parallel()
	for lname, load := range loaders() {
		src := validFor(lname)
		t.Run(lname, func(t *testing.T) {
			t.Parallel()
			pad := maxFileBytes - len(src) - len("#\n")
			exact := src + "#" + strings.Repeat("x", pad) + "\n"
			require.Len(t, exact, maxFileBytes)
			require.NoError(t, load([]byte(exact)))
			require.ErrorContains(t, load([]byte(exact+"#\n")), "too large")
		})
	}
}

func TestParseRejectsYAMLFeatures(t *testing.T) {
	t.Parallel()
	entry := func(summary string) string { return flowEntry(summary, "false", "1") }
	for name, tc := range map[string]struct{ src, want string }{
		"class merge":       {doc("classes: {<<: {internal: ! 1}}", "codes: []"), notAllowed},
		"class plain merge": {doc("classes: {<<: {internal: 1}}", "codes: []"), notAllowed},
		"top merge class":   {doc("<<: {classes: {internal: 1}}", "codes: []"), notAllowed},
		"entry merge": {doc("classes: {internal: 1}", "codes:",
			"  - {<<: {code: a.b}, class: internal, retryable: false, exit: 1, summary: s}"), notAllowed},
		"anchor key": {doc("classes: {internal: 1}", "codes:",
			"  - {&f code: a.b, class: internal, retryable: false, exit: 1, summary: s}"), notAllowed},
		"alias key": {doc("classes: {internal: 1}", "codes:",
			"  - {&f code: a.b, class: internal, retryable: false, exit: 1, summary: s}",
			"  - {*f : a.c, class: internal, retryable: false, exit: 1, summary: s}"), notAllowed},
		"binary key": {doc("classes: {internal: 1}", "codes:",
			"  - {code: a.b, class: internal, retryable: false, summary: s, !!binary ZXhpdA==: ! 1}"), notAllowed},
		"merge reuse": {doc("classes: {internal: 1}", "codes:",
			"  - &base {code: a.b, class: internal, retryable: false, exit: ! 1, summary: s}",
			"  - {<<: *base, code: a.c}"), notAllowed},
		"anchor summary": {entry("&s x"), notAllowed},
		"alias summary": {doc("classes: {internal: 1}", "codes:",
			"  - {code: a.b, class: internal, retryable: false, exit: 1, summary: &s x}",
			"  - {code: a.c, class: internal, retryable: false, exit: 1, summary: *s}"), notAllowed},
		"anchor entry": {doc("classes: {internal: 1}", "codes:",
			"  - &e {code: a.b, class: internal, retryable: false, exit: 1, summary: s}"), notAllowed},
		"alias entry": {doc("classes: {internal: 1}", "codes:",
			"  - &e {code: a.b, class: internal, retryable: false, exit: 1, summary: s}",
			"  - *e"), notAllowed},
		"str tag summary":    {entry("!!str x"), notAllowed},
		"custom tag summary": {entry("!custom x"), notAllowed},
		"binary summary":     {entry("!!binary /w=="), notAllowed},
		"bare tag summary":   {entry("! x"), notAllowed},
		"bare tag classes":   {doc("classes: ! {internal: 1}", "codes: []"), notAllowed},
		"bare tag codes": {doc("classes: {internal: 1}", "codes: !",
			"  - {code: a.b, class: internal, retryable: false, exit: 1, summary: s}"), notAllowed},
		"bare tag key": {doc("classes: {internal: 1}", "codes:",
			"  - {! code: a.b, class: internal, retryable: false, exit: 1, summary: s}"), notAllowed},
		"int key": {
			doc("classes: {internal: 1}", "codes:",
				"  - {code: a.b, class: internal, retryable: false, exit: 1, summary: s, 7: x}"),
			plainKeys,
		},
		"bool key": {
			doc("classes: {internal: 1}", "codes:",
				"  - {code: a.b, class: internal, retryable: false, exit: 1, summary: s, true: x}"),
			plainKeys,
		},
		"complex key": {
			doc("classes: {internal: 1}", "codes:",
				"  - {code: a.b, class: internal, retryable: false, exit: 1, summary: s, [x]: y}"),
			plainKeys,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := registry.Parse([]byte(tc.src))
			require.ErrorContains(t, err, tc.want)
		})
	}
}

func TestLoadInventoryRejectsYAMLFeatures(t *testing.T) {
	t.Parallel()
	for name, src := range map[string]string{
		"class merge":    publishedDoc("{<<: {internal: ! 1}}", "false"),
		"top merge":      doc("<<: {classes: {internal: 1}}", "codes: []"),
		"anchor":         publishedDoc("{internal: &a 1}", "false"),
		"alias":          publishedDoc("{internal: &a 1, config: *a}", "false"),
		"tag":            publishedDoc("{internal: !custom 1}", "false"),
		"retryable anch": publishedDoc("{internal: 1}", "&a false"),
		"bare tag":       publishedDoc("! {internal: 1}", "false"),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := registry.LoadInventory(strings.NewReader(src))
			require.ErrorContains(t, err, notAllowed)
		})
	}
}

func TestParseKeepsBangInBlockScalars(t *testing.T) {
	t.Parallel()
	for name, summary := range map[string]string{
		"literal": "|-\n      ! line",
		"folded":  ">-\n      # ! not a comment",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			src := doc(
				"classes: {internal: 1}",
				"codes:",
				"  - code: a.b",
				"    class: internal",
				"    retryable: false",
				"    exit: 1",
				"    summary: "+summary,
			)
			_, err := registry.Parse([]byte(src))
			require.NoError(t, err)
		})
	}
}

func TestCheckSummaryRejectsInvalidUTF8(t *testing.T) {
	t.Parallel()
	require.ErrorContains(t, registry.CheckSummary("a.b", "x\xffy"), "valid UTF-8")
	require.NoError(t, registry.CheckSummary("a.b", "x y"))
}

func TestLoadInventoryReadsPublishedFile(t *testing.T) {
	t.Parallel()
	f, err := os.Open("../../../schema/errors.published.yaml")
	require.NoError(t, err)
	t.Cleanup(func() { _ = f.Close() })
	inventory, err := registry.LoadInventory(f)
	require.NoError(t, err)
	require.True(t, inventory.HasClassExits())
}
