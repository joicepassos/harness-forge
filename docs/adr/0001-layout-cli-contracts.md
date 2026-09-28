# ADR 0001: Layout, compatibility, and CLI contracts

- Status: accepted; implemented contract
- Date: 2026-09-27

## Context

HarnessForge must keep existing `.harness/harness.yaml` v1/v2 projects working
while introducing `.forge/forge.yaml`. Selecting between divergent sources by
filesystem order would make validation and publication unpredictable. CI also
needs stable machine-readable diagnostics and exit codes. Several independent
versions exist in this contract; using an unqualified word such as “version”
for all of them invites accidental coupling.

## Decisions

1. Version fields are independent contracts:
   - `.harness/harness.yaml` `version` is the legacy Harness IR version. Readers
     support 1 and 2.
   - `.forge/forge.yaml` `layout_version` versions the Forge manifest/layout;
     version 1 is supported. Its `ir_version` selects the canonical Harness IR
     represented by its contents; readers support 1 and 2.
   - `harnessforge check --format json` envelope `version` versions that output
     contract; version 1 is current. It does not indicate the project IR or
     executable release.
   - `harnessforge version` reports build metadata, not a schema version.
   Unknown layout or IR versions fail validation; they are never guessed or
   silently upgraded. A compatible additive JSON envelope field may be added
   within version 1. Renaming/removing/changing the meaning of a field or
   changing required envelope semantics requires a new JSON format version.
2. Existing `.harness/harness.yaml` remains readable during migration. When only
   one supported layout exists, automatic discovery may select it. When both
   exist, commands that discover a source must fail with a conflict unless the
   caller explicitly selects `--layout harness` or `--layout forge`. Commands
   accepting an explicit config-file path use that path as their input; they do
   not rewrite it or use it to silently resolve another layout.
3. Paths stored in either layout are repository-relative, use `/` separators,
   and resolve relative to the repository root. Load and validate operations
   are offline and never execute project commands.
4. `harnessforge check --format json` is the versioned diagnostics interface.
   Its version-1 envelope has required `version`, `ok`, and `diagnostics`
   fields, plus optional `gate_results`. Each diagnostic has stable `code`,
   `severity`, and `message`, with optional repository-relative `path` and
   `suggestion`. Diagnostics are data, not localized identifiers: consumers
   branch on `code` and severity, not message text. The command emits one JSON
   document to stdout; human-readable output remains the default. Other CLI
   commands may have JSON output but are not covered by this envelope unless
   they explicitly document it.
5. CLI process exit codes are: 0 when the command succeeds (including all
   required checks passing), 1 when a check completes and reports a failed
   requirement, and 2 for usage, configuration, or execution errors. A check
   required by the selected invocation/profile but not run is a failure, not a
   pass. In `check --format json`, a completed check failure is represented by
   `ok: false` and diagnostics and exits 1; inability to complete the command
   exits 2. Do not infer check status from localized text or gate subprocess
   exit-code numbers in `gate_results`.
6. Migrations use preview before application, preserve the source, and report
   fields that cannot be mapped. They never dual-write the old and new layouts.

## Consequences

The resolver is shared by CLI and validation code. Consumers can pin a layout
explicitly during coexistence, while projects with one layout keep concise
commands. Schema changes must update the matching schema, fixtures, and
compatibility tests. Adding a future layout or diagnostic format requires a
versioned contract rather than changing the meaning of existing fields.

## Verification

Acceptance fixtures cover one layout, both layouts with an explicit selection,
both layouts without a selection, relative references, JSON version 1, and the
exit-code classes. `cmd/harnessforge/exitcode_test.go` exercises failed checks
(1) and configuration errors (2). Successful command execution returns
normally (0); the process-level helper currently does not assert that class.
Check command tests cover JSON serialization and check status. Schema and
resolver tests are deterministic and make no provider calls.
