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

`schema/errors.yaml` lists every error code with its class, retry flag, exit code and a one-line summary. Codes are lowercase `domain.reason`, plus the standalone codes `unknown`, `canceled` and `timeout`. They are never renamed and never reused. `codes/codes_gen.go` and `ts/src/codes.ts` are generated from it; do not edit them. After changing the registry run:

```sh
cd tools && go run ./cmd/gencodes
```

CI fails when the generated files differ from a fresh run. The `tools` directory is a separate Go module, so the root module keeps no dependencies. The generator needs Go 1.24 or newer.

## Versioning

Tags follow semver. Before v1.0.0, a minor release may break the Go API; the envelope `v` field changes only for a breaking wire change.

## License

MPL-2.0. See `LICENSE`.
