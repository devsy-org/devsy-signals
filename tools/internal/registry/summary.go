package registry

import (
	"fmt"
	"strings"
	"unicode"
)

const lineBreaks = "\r\n\v\f\u0085\u2028\u2029"

func checkSummary(code, summary string) error {
	if strings.TrimSpace(summary) == "" {
		return fmt.Errorf("code %q summary must not be blank", code)
	}
	if strings.ContainsAny(summary, lineBreaks) {
		return fmt.Errorf("code %q summary must be a single line", code)
	}
	if strings.IndexFunc(summary, unsafeRune) >= 0 {
		return fmt.Errorf(
			"code %q summary must be plain text: no control characters and no invisible "+
				"characters (Unicode Default_Ignorable_Code_Point, which includes bidirectional "+
				"controls, zero-width characters including the joiner U+200D, variation selectors, "+
				"tag characters, U+180E, U+3164 and the byte order mark U+FEFF)", code)
	}
	return nil
}

// unsafeRune reports control characters and invisible characters. A summary
// is plain text, so emoji sequences that need U+200D or a variation selector
// are refused on purpose.
func unsafeRune(r rune) bool {
	return unicode.IsControl(r) || defaultIgnorable(r)
}

// defaultIgnorable reports the Unicode Default_Ignorable_Code_Point property.
// Go's unicode package has no table for it, so this follows the derivation in
// DerivedCoreProperties.txt: Other_Default_Ignorable_Code_Point, Cf and
// Variation_Selector, minus White_Space, the interlinear annotation characters
// U+FFF9 to U+FFFB, the Egyptian format controls U+13430 to U+13440 and the
// Prepended_Concatenation_Mark characters. The tables follow the Go toolchain's
// unicode.Version, which differs between toolchains, so a code point Unicode
// assigns later may be missed until the toolchain updates.
func defaultIgnorable(r rune) bool {
	if !unicode.In(
		r,
		unicode.Other_Default_Ignorable_Code_Point,
		unicode.Cf,
		unicode.Variation_Selector,
	) {
		return false
	}
	switch {
	case unicode.Is(unicode.White_Space, r),
		r >= 0xfff9 && r <= 0xfffb,
		r >= 0x13430 && r <= 0x13440:
		return false
	}
	return !prependedConcatenationMark(r)
}

func prependedConcatenationMark(r rune) bool {
	switch {
	case r >= 0x0600 && r <= 0x0605, r >= 0x0890 && r <= 0x0891:
		return true
	}
	switch r {
	case 0x06dd, 0x070f, 0x08e2, 0x110bd, 0x110cd:
		return true
	}
	return false
}
