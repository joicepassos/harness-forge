# ADR 0001: Layout, compatibility, and CLI contracts

- Status: accepted for the P1 implementation
- Date: 2026-09-27

## Context

HarnessForge must keep existing `.harness/harness.yaml` v1/v2 projects working
while introducing `.forge/forge.yaml`. Selecting between divergent sources by
filesystem order would make validation and publication unpredictable. CI also
needs stable machine-readable diagnostics and exit codes.

## Decisions

1. Harness IR `version` remains its own compatibility version (currently 1 or
   2). `.forge/forge.yaml` uses the separate `layout_version` field; its first
   supported value is 1 and it declares the IR through `ir_version`.
2. Existing `.harness/harness.yaml` remains readable during migration. When only
   one supported layout exists, automatic discovery may select it. When both
   exist, commands that discover a source must fail with a conflict unless the
   caller explicitly selects `--layout harness` or `--layout forge`. An explicit
   `--file` path takes precedence and is never silently rewritten.
3. Paths stored in either layout are repository-relative, use `/` separators,
   and resolve relative to the repository root. Load and validate operations
   are offline and never execute project commands.
4. Structured check diagnostics use JSON format version 1. The envelope has
   `version`, `ok`, and `diagnostics`; each diagnostic has stable `code`,
   `severity`, `message`, and optional repository-relative `path` and
   `suggestion`. Human-readable output remains the default.
5. CLI exit codes are: 0 when all required checks pass, 1 when a check fails,
   and 2 for usage, configuration, or execution errors. A check that is
   required but was not run is a failure, not a pass.
6. Migrations use preview before application, preserve the source, and report
   fields that cannot be mapped. They never dual-write the old and new layouts.

## Consequences

The resolver is shared by CLI and validation code. Consumers can pin a layout
explicitly during coexistence, while projects with one layout keep concise
commands. Adding a future layout or diagnostic format requires a versioned
contract rather than changing the meaning of existing fields.

## Verification

Acceptance fixtures cover one layout, both layouts with an explicit selection,
both layouts without a selection, relative references, JSON version 1, and the
three exit-code classes. Schema and resolver tests are deterministic and make no
provider calls.
