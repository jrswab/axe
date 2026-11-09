# Compatibility Policy

This file is the compatibility promise for axe v1 releases. It follows Go 1 style: every v1 release keeps the rules below, and a change that breaks them waits for a new major version.

## pkg/runner exported API

- No exported identifier is removed or changed in v1.
- Additions are allowed.
- Callers should use keyed struct literals, so that added fields do not break them.
- Callers must not compare exported structs with `==`, because added fields can change that comparison.

## CLI flags and exit codes

- Changes are additive only. New flags and new exit codes may be added.
- Existing flags, their meaning, and existing exit codes are not removed or changed in v1.

## Agent TOML keys

- Changes are additive only. New keys may be added.
- Existing keys, their meaning, and their accepted values are not removed or changed in v1.

## `--json` output fields

- Changes are additive only. New fields may be added.
- Existing fields keep their names, types, and meaning in v1.

## Default runtime behavior

- Default runtime behavior is unchanged except for documented bug fixes.

## Behavior changes are opt-in

- New behavior is opt-in through a new TOML key or a new `Options` field.
- The zero value of every new key or field keeps today's behavior.
- No GODEBUG-style switch is used in v1.

## Enforcement (planned)

- An apidiff check compares `pkg/runner` against the `v1.11.0` tag. It runs in `make check` and in CI.
- The check is invoked as `go run golang.org/x/exp/cmd/apidiff@<pinned version>`. It is not added to `go.mod`.
- The gate lands in milestone 002. It is not active today.
- The pin is recorded when the gate lands. It is a concrete version, not `latest`.

## Deprecation

- In v1, a symbol, key, flag, or field is deprecated, never removed.
- Removal waits for `/v2`.

## Pre-policy exceptions

These changes shipped before this policy. They are recorded here and are not reverted.

1. **1.11.0 `temperature = 0` behavior change.** OpenAI and Atlas Cloud send `"temperature": 0` when agent TOML sets `temperature = 0`. This shipped in a minor release with no opt-out. See `CHANGELOG.md:24`.
2. **Minimum Go raised to 1.26.0.** Milestone 000 raised the `go` directive. `go.mod:3` is `go 1.26.0`, and `go.mod:5` is `toolchain go1.26.10`. The prior minimum was 1.25.0. Importers on Go 1.25 cannot build `pkg/runner` after this change.
