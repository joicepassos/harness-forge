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
- T5.1/T5.6: read-only aggregate `check`, static Forge/Harness output drift,
  optional explicit gate execution, and CI drift workflow are implemented.
- T5.2–T5.4: explicit candidate import, reviewer-bound approval with evidence
  revalidation, local observation promotion, and knowledge checks are covered.
- T5.5: shared context preview and proposal generation exist; interactive
  project wizard and conflict-resolution UX remain incomplete.
- T6.1–T6.4: approved knowledge context, scope/path selection, explicit budget
  overflow, provenance, and comparison proxies are implemented. Quality remains
  unmeasured; the metrics are deterministic proxies.
- T7.1–T7.5: explicit local capture, review, candidate publication, retention,
  quota, integrity, and checkout isolation are implemented.
- T8.1/T8.2: official-doc behavior matrices exist; no Cursor/OpenCode binary
  runtime tests were performed, so release acceptance remains open.
- T8.3: transport-neutral read-only context resolver is implemented. JSON-RPC,
  session, authentication, and client tests remain out of scope.
- T8.4: portable skill publication contract is documented; sync keeps
  references rather than duplicating skill bodies.
- T8.5: opt-in runner supports workspace, timeout, cancellation and bounded
  output. T8.6 CI execution on selected real project gates remains to validate.
- T9.1–T9.5: pilot protocol and run schema are documented; no pilot runs or
  measurements exist. T9.6 runtime decision is deferred pending evidence.

## Verification

Passed:

```text
go test ./...
go vet ./...
```

The local full test suite and static analysis pass. The cross-platform CI matrix
has not yet been run. No paid provider calls were made.

## Remaining validation

- External Cursor/OpenCode/Codex/Claude runtime validation, end-to-end clone
  agent runs, pilot measurements, and the runtime decision require pinned agent
  builds and representative repositories; none are fabricated here.
- This checkout contains pre-existing repository-context/retrieval changes;
  they remain outside the Forge plan task commits.
