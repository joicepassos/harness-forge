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
  Forge-to-legacy generation now fails closed with directions to native sync,
  rather than emitting a partial Harness. Versioned migration round-trip
  fixtures pass; broader field-level parity beyond mapped fields remains open.
- T3.1–T3.4: migration preview reports unmapped choices, apply preserves the
  legacy source, rollback protects modified/unowned output, and v1 migration
  remains compatible. Skill evidence now survives conversion through the
  versioned manifest and schema. Cross-layout legacy command compatibility is
  explicit: legacy `generate` directs Forge projects to native `sync` rather
  than emitting a partial document.
- T4.1–T4.6: deterministic Codex/Claude compile adapters, shared ownership
  manifest, dry-run/check/apply, collision and edit protection, staging, clone
  ownership, idempotence, and symlink checks are implemented. T4.5 now records a
  durable recovery journal before publishing files; interrupted transactions
  roll back on the next apply, and post-interruption human edits are preserved
  with an explicit conflict. Native target scope semantics and concurrent
  writers remain unverified.
- T5.1/T5.6: read-only aggregate `check`, static Forge/Harness output drift,
  optional explicit gate execution, and CI drift workflow are implemented.
  CLI exit codes follow ADR 0001: success 0, failed checks 1, and usage,
  configuration, or execution errors 2. The cross-platform workflow runs both
  no-Forge clone gates and verifies generated drift. It does not launch an
  external agent against the clone, so runtime discovery remains unverified.
- T5.2–T5.4: explicit candidate import, reviewer-bound approval bound to rule,
  evidence, and reviewable metadata hashes, and Forge knowledge/evidence drift
  are covered. Legacy approvals without the metadata digest require re-review.
  `drift` reports
  missing or changed evidence separately and always leaves semantic conformance
  `not_evaluated`; evidence presence is not treated as proof of conformance.
- T5.5: `harnessforge onboard` provides text/JSON guidance, explicit handling of
  missing/invalid/ambiguous layouts, candidate import, review-before-export
  steps, and safe sync guidance. It is command-driven rather than an interactive
  wizard; model-backed proposals remain optional.
- T6.1–T6.2: approved knowledge is selected with content/evidence and review
  metadata hash validation, freshness checks, provenance, scope, explicit
  keywords, and task-path glob matching. The CLI exposes task paths. Backend /
  frontend `Build` integration coverage verifies both selection and explicit
  out-of-scope exclusions. T6.3 reports selected-item and early prompt-envelope
  overflow with the configured estimator and an explicit excluded-prompt reason.
  T6.4 exposes comparison proxies; task-level quality remains unmeasured.
- T7.1–T7.5: local observation capture, review, candidate publication,
  audit provenance, deduplication, and checkout isolation are implemented and
  covered. T7.4 supports preview/apply, age and quota retention, pending-item
  preservation, atomic snapshot replacement, and safe retry after an injected
  interruption; pending items survive and repeated collection is idempotent.
- T8.1/T8.2: official-doc behavior matrices exist; no Cursor/OpenCode binary
  runtime tests were performed, so release acceptance remains open. The
  snapshots separate documented behavior from exact-build runtime evidence.
- T8.3: the read-only context resolver is exposed through MCP JSON-RPC stdio
  for protocol `2025-06-18`, with initialize, resources/list, resources/read,
  a task-prompt resource template, optional repository-relative task paths,
  bounded messages, and no tools or write operations. Protocol behavior is
  covered locally; remote transport, authentication, and client-specific
  discovery remain out of scope. See [MCP context contract](MCP-CONTEXT.md).
- T8.4: sync validates and publishes portable Agent Skills bundles under
  `.agents/skills/` and `.claude/skills/`; generated instruction files link to
  skills without duplicating their bodies. Updates and stale-file cleanup use
  generated-manifest hashes, and symlinks/special files are rejected.
- T8.5: the opt-in gate runner supports workspace, timeout, cancellation and
  bounded output. Cancellation terminates the shell process tree on supported
  operating systems. Policies marked `enforced` fail closed because no policy
  enforcement executor is implemented. T8.6 adds a dedicated CI workflow that
  verifies Forge sync and runs the repository's `go test ./...` gate; both
  commands passed locally. The workflow has not yet run on GitHub.
- T9.1: the versioned inventory records Mili, PromptForge, and Toca as candidate
  commits, and a 12-task draft is available. The user approved MLI-01–05,
  Codex CLI's local default configuration, and three repetitions. Pilot
  conditions and the Forge source/export still need a clean, reproducible freeze;
  the Mili checkout's current context/index files are untracked and derive from
  a modified tree. See [the partial task approval](pilot/mili-task-approval-v1.json).
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
- T0.4 and T9.2–T9.5 need the remaining condition/export freeze and independent
  evaluator rubric. Mili's clean HEAD and the five approved task texts are
  pinned; its modified code working tree is excluded. The available Go
  fixture is only development data and is not treated as pilot evidence.
- T6.2 now supports explicit knowledge keywords and task paths through the
  context API and `context explain --task-path`; glob matching is segment-aware
  with `**`, and backend/frontend scope fixtures pass. T6.3 reports early
  prompt-envelope overflow with the same configured estimator and an explicit
  excluded-prompt reason. T6.4 comparison proxies must not be described as
  measured task quality.
- T1.7/T2/T3/T4/T5 also retain acceptance gaps listed in their status entries;
  static fixtures and repository-level tests are not external runtime evidence.
- This checkout contains pre-existing repository-context/retrieval changes;
  they remain outside the Forge plan task commits.
