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
- T2.2–T2.6: `.forge/forge.yaml` has a versioned contract independent from
  Harness IR v1/v2; offline loading, reference validation, and v1/v2 round-trip
  compatibility are covered.
- T3.1–T3.4: migration preview reports unmapped choices, apply preserves the
  legacy source, rollback protects modified/unowned output, and v1 migration
  remains compatible.
- T4.1–T4.6: deterministic Codex/Claude compile adapters, shared ownership
  manifest, dry-run/check/apply, collision and edit protection, staging, clone
  ownership, idempotence, and symlink checks are implemented.
- T5.1/T5.6 scope delivered: `validate` discovers the selected project layout;
  static clone fixtures prove generated instructions are consumable without
  Forge configuration. Aggregate `check`, project onboarding/import and CI
  drift facade remain follow-up work.

## Verification

Passed:

```text
go test ./...
```

`go vet ./...` and the cross-platform CI matrix have not yet been run. No paid
provider calls were made.

## Remaining MVP work

- T5.2–T5.5: explicit candidate import/review and onboarding; broader aggregate
  diagnostics and drift integration.
- T5.6 remainder: wire the static sync check into CI and verify the complete
  maintenance workflow in an external clone.

P4–P8 remain later work as the plan specifies. The repository-context edits that
pre-dated this implementation remain uncommitted and are excluded from these
task commits.
