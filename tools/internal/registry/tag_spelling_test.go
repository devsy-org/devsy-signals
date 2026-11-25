package registry_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	tagMarker = "@@"

	loaderParse     = "parse"
	loaderInventory = "inventory"
	posRoot         = "root"
	posClassExit    = "class exit"
	posRetryable    = "retryable"
	posEntry        = "entry"
)

// tagDocs returns, for each loader, one document per node position with the
// tag marker in place. The "root" document puts the marker on the root mapping.
func tagDocs() map[string]map[string]string {
	return map[string]map[string]string{
		loaderParse: {
			posRoot:      tagMarker + "\n" + blockEntry("false", "1"),
			posClassExit: doc("classes: {internal: "+tagMarker+" 1}", "codes: []"),
			posRetryable: doc(
				"classes: {internal: 1}",
				"codes:",
				"  - {code: a.b, class: internal, retryable: "+tagMarker+" false, exit: 1, summary: s}",
			),
			posEntry: doc(
				"classes: {internal: 1}",
				"codes:",
				"  - "+tagMarker+" {code: a.b, class: internal, retryable: false, exit: 1, summary: s}",
			),
		},
		loaderInventory: {
			posRoot:      tagMarker + "\n" + publishedDoc("{internal: 1}", "false"),
			posClassExit: publishedDoc("{internal: "+tagMarker+" 1}", "false"),
			posRetryable: publishedDoc("{internal: 1}", tagMarker+" false"),
			posEntry: doc("classes: {internal: 1}", "codes:",
				"  - "+tagMarker+" {code: a.b, class: internal, retryable: false, status: active}"),
		},
	}
}

func TestRejectsEveryTagSpelling(t *testing.T) {
	t.Parallel()
	const directive = "%TAG !e! tag:example.com,2000:\n"
	for spell, header := range map[string]string{
		"!":                        "",
		"!<!>":                     "",
		"!<%21>":                   "",
		"!<tag:yaml.org,2002:int>": "",
		"!<tag:yaml.org,2002:str>": "",
		"!!int":                    "",
		"!custom":                  "",
		"!e!x":                     directive,
	} {
		for lname, positions := range tagDocs() {
			for pos, template := range positions {
				t.Run(lname+" "+pos+" "+spell, func(t *testing.T) {
					t.Parallel()
					src := header + "---\n" + strings.Replace(template, tagMarker, spell, 1)
					if pos == posRoot {
						src = header + "--- " + strings.Replace(template, tagMarker, spell, 1)
					}
					require.ErrorContains(t, loaders()[lname]([]byte(src)), notAllowed)
				})
			}
		}
	}
}
