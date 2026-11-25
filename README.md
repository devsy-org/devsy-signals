# devsy-signals

Shared observability contract for Devsy: the record envelope, error codes, redaction and the provider conformance runner. Core, the runtime SDK, providers and Desktop pin released tags of this repository, so none of them depends on unreleased Core code.

## Layout

| Path | Content |
|---|---|
| `schema/` | Envelope JSON Schema, per-type payload schemas, `errors.yaml` |
| repo root | Go module `github.com/devsy-org/devsy-signals` (stdlib only, Go 1.23 or newer) |
| `redact/` | Redaction rules shared by every Devsy sink |
| `fixtures/` | Redaction corpus and envelope golden files, run by the Go and TypeScript tests |
| `ts/` | TypeScript package for Desktop |
| `cmd/devsy-conformance` | Provider conformance runner |

## Error registry

`schema/errors.yaml` lists every error code with its class, retry flag, exit code and a one-line summary. Codes are lowercase `domain.reason`, plus the standalone codes `unknown`, `canceled` and `timeout`. They are never renamed and never reused. The registry and inventory files are plain UTF-8 YAML data of at most 1 MiB: no anchors, aliases, tags or merge keys, and every mapping key is a string. A summary is one line of plain text: no control characters and no characters with the Unicode Default_Ignorable_Code_Point property (zero-width characters including U+200D, bidirectional controls, variation selectors, tag characters, U+3164, U+FFA0, U+FEFF). Emoji sequences that need a joiner or variation selector are refused. `codes/codes_gen.go` and `ts/src/codes.ts` are generated from it; do not edit them. After changing the registry run:

```sh
cd tools && go run ./cmd/gencodes
```

CI fails when the generated files differ from a fresh run. CI also runs `go run ./cmd/checkpublished -base <sha>` in `tools`, which keeps `schema/errors.published.yaml` append-only. On a pull request the base is the PR base commit; on a push to `main` it is the previous `main` commit (`github.event.before`), and the check is skipped when that is the all-zero sha. The `tools` directory is a separate Go module, so the root module keeps no dependencies. The generator needs Go 1.24 or newer.

## Versioning

Tags follow semver. Before v1.0.0, a minor release may break the Go API; the envelope `v` field changes only for a breaking wire change.

## License

MPL-2.0. See `LICENSE`.
