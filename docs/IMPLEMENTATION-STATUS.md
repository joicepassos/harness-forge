# Implementation status

Date: 2026-09-27. Branch: `codex/forge-evolution-mvp`. Base: `bb226d8`.

## Completed in this implementation

- T0.1: this status record captures the branch, current verification, and open
  work. The checkout also contained pre-existing repository-context changes;
  those are still uncommitted and are not claimed as part of this plan's tasks.
- T0.5: [ADR 0001](adr/0001-layout-cli-contracts.md) defines layout coexistence,
  relative references, JSON diagnostics version, and CLI exit codes.
- T1.1–T1.5: generation preserves skills and structured workspaces; generated
  files use ownership hashes; approval records bind rule/evidence fingerprints;
  discovery rejects conflicting IDs; drift reports evidence presence and
  coverage separately from conformance.
- T1.6: `testdata/clones/without-forge/` contains Codex and Claude Code clones
  with static native instructions and no HarnessForge configuration.
- T1.7: acceptance tests for generation, ownership, review, discovery, and drift
  passed; see [P0–T1 acceptance](acceptance/p0-t1.md).
- T2.1: a shared layout resolver discovers `.harness` or `.forge`, rejects
  ambiguous coexistence unless selection is explicit, and resolves safe
  repository-relative paths.
- T2.2: `.forge/forge.yaml` has a versioned contract whose `layout_version` is
  independent from Harness IR v1/v2.

## Verification

Passed:

```text
go test ./internal/generation/... ./internal/harness/... ./internal/discovery/... ./internal/drift/... ./cmd/harnessforge
go test ./internal/harness/infrastructure ./internal/harness/domain
git diff --check
```

The full `go test ./...`, `go vet ./...`, and cross-platform CI matrix have not
yet been run for this implementation branch. No paid provider calls were made.

## Remaining MVP work

- T2.3–T2.6: load `.forge` offline, validate references, and cover round-trip and
  source conflicts.
- T3: reversible migration preview/application and compatibility acceptance.
- T4: reproducible compile plan, versioned manifest, Codex/Claude adapters, and
  safe `sync --dry-run`, `sync --check`, and apply.
- T5: aggregated `check`, candidate import/review, onboarding, and CI drift
  verification. These close the P0–P3 MVP.

P4–P8 remain later work as the plan specifies. The current layout resolver is an
internal API; CLI commands do not yet use it for automatic source selection.
