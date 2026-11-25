# Envelope schema

`envelope.schema.json` references `audit-data.schema.json`, generated from `audit-allowlist.json` by `go run ./cmd/genauditschema` in `tools/`. Load both schemas when validating.

`tools/schematest/go_only.json` lists schema-valid fixtures that Go must reject, with a one-line reason for each exception. The differential test requires both verdicts for each listed fixture and requires agreement for every unlisted fixture.
