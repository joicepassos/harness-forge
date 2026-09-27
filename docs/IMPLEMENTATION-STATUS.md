# Implementation status

Date: 2026-09-27. Branch: `codex/forge-evolution-mvp`. Base: `bb226d8`.

## Completed in this implementation

- T0.1: this status record captures the branch, current verification, and open
  work. The checkout also contained pre-existing repository-context changes;
  those are still uncommitted and are not claimed as part of this plan's tasks.
- T0.2: the six priority findings are mapped to regression tests in
  [the acceptance map](acceptance/t0.2-regressions.md).
- T0.3: reusable single-module and Go-workspace monorepo fixtures cover
  repository paths and workspace roots. Windows path semantics are covered by
  host-independent resolver tests, not by a Windows filesystem fixture.
- T0.5: [ADR 0001](adr/0001-layout-cli-contracts.md) defines layout coexistence,
  relative references, JSON diagnostics version, and CLI exit codes.
- T1.1–T1.5: generation preserves skills and structured workspaces; generated
  files use ownership hashes; approval records bind rule/evidence fingerprints;
  discovery rejects conflicting IDs; drift reports evidence presence and
  coverage separately from conformance.
- T1.6: `testdata/clones/without-forge/` contains Codex and Claude Code clones
  with static native instructions and no HarnessForge configuration.
- T1.7: deterministic repository-level tests for generation, ownership, review,
  discovery, and drift passed; external clone-agent acceptance is not complete.
  The static clone fixture proves only that native instruction files are
  present; see [P0–T1 acceptance](acceptance/p0-t1.md).
- T2.1: a shared layout resolver discovers `.harness` or `.forge`, rejects
  ambiguous coexistence unless selection is explicit, and resolves safe
  repository-relative paths.
- T2.2–T2.6: `.forge/forge.yaml` has a versioned contract independent from
  Harness IR v1/v2; offline loading and reference validation are covered.
  Full canonical field parity is incomplete: legacy `generate` can receive a
  partial Harness when loading Forge, and v1/v2 round-trip claims need broader
  field-level coverage.
- T3.1–T3.4: migration preview reports unmapped choices, apply preserves the
  legacy source, rollback protects modified/unowned output, and v1 migration
  remains compatible. Skill evidence is currently dropped during conversion;
  cross-layout legacy command compatibility is not fully demonstrated.
- T4.1–T4.6: deterministic Codex/Claude compile adapters, shared ownership
  manifest, dry-run/check/apply, collision and edit protection, staging, clone
  ownership, idempotence, and symlink checks are implemented. Native target
  scope semantics, concurrent writers, and crash/restart recovery remain
  unverified.
- T5.1/T5.6: read-only aggregate `check`, static Forge/Harness output drift,
  optional explicit gate execution, and CI drift workflow are implemented.
  CLI exit codes do not yet match ADR 0001's 0/1/2 contract, and the workflow
  does not run an agent against the no-Forge clone.
- T5.2–T5.3: explicit candidate import and reviewer-bound approval with
  evidence revalidation are covered. T5.4 remains partial: Forge knowledge and
  evidence drift are not integrated into the `drift` command.
- T5.5: `harnessforge onboard` provides text/JSON guidance, explicit handling of
  missing/invalid/ambiguous layouts, candidate import, review-before-export
  steps, and safe sync guidance. It is command-driven rather than an interactive
  wizard; model-backed proposals remain optional.
- T6.1: approved knowledge is selected with content/evidence hash validation,
  freshness checks, provenance, and scope metadata. T6.2 is partial: scope and
  content use lexical matching; explicit keywords, path/glob evaluation against
  a task path, and backend/frontend build-level selection coverage are missing.
  T6.3 reports budget overflow for selected knowledge, but early prompt-envelope
  overflow lacks complete exclusion details. T6.4 exposes comparison proxies;
  task-level quality and an independent baseline remain unmeasured.
- T7.1–T7.3 and T7.5: local observation capture, review, candidate publication,
  audit provenance, deduplication, and checkout isolation are implemented and
  covered. T7.4 supports preview/apply, age and quota retention, and pending-item
  preservation; interruption/recovery/resume behavior is not demonstrated.
- T8.1/T8.2: official-doc behavior matrices exist; no Cursor/OpenCode binary
  runtime tests were performed, so release acceptance remains open. The
  snapshots separate documented behavior from exact-build runtime evidence.
- T8.3: transport-neutral read-only context resolver is implemented. JSON-RPC,
  session, authentication, and client tests remain out of scope.
- T8.4: sync validates and publishes portable Agent Skills bundles under
  `.agents/skills/` and `.claude/skills/`; generated instruction files link to
  skills without duplicating their bodies. Updates and stale-file cleanup use
  generated-manifest hashes, and symlinks/special files are rejected.
- T8.5: the opt-in gate runner supports workspace, timeout, cancellation and
  bounded output. Policies marked `enforced` fail closed because no policy
  enforcement executor is implemented. T8.6 adds a dedicated CI workflow that
  verifies Forge sync and runs the repository's `go test ./...` gate; both
  commands passed locally. The workflow has not yet run on GitHub.
- T9.1: the versioned inventory records Mili, PromptForge, and Toca as candidate
  commits, and a 12-task draft is available for review. User task approval,
  agent/model configuration, and the run/evaluation protocol are not frozen.
  T0.4 and T9.2–T9.5 remain pending; T9.6 is deferred until comparative
  evidence exists.

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
- T0.4 and T9.2–T9.5 need a frozen validation corpus, executable tasks, pinned
  agent/model configurations, and human review. Mili's clean HEAD is available
  as one candidate; its modified working tree is excluded. The available Go
  fixture is only development data and is not treated as pilot evidence.
- T6.2/T6.3 and T7.4 retain the specific coverage gaps above; T6.4 comparison
  proxies must not be described as measured task quality.
- T1.7/T2/T3/T4/T5 also retain acceptance gaps listed in their status entries;
  static fixtures and repository-level tests are not external runtime evidence.
- This checkout contains pre-existing repository-context/retrieval changes;
  they remain outside the Forge plan task commits.
