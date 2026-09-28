# Implementation status

Date: 2026-09-28. Branch: `codex/forge-evolution-mvp`. Base: `bb226d8`.

## Completed in this implementation

- T0.1: this status record captures the branch, current verification, and open
  work. The checkout also contained pre-existing repository-context changes;
  those are still uncommitted and are not claimed as part of this plan's tasks.
- T0.2: the six priority findings are mapped to regression tests in
  [the acceptance map](acceptance/t0.2-regressions.md).
- T0.3: reusable single-module, Go-workspace monorepo, and Windows path/workspace
  fixtures cover repository paths, portable references, and workspace roots.
- T0.5: [ADR 0001](adr/0001-layout-cli-contracts.md) defines independent layout,
  IR, JSON diagnostic, and build versions, compatibility behavior, and CLI exit
  codes, aligned with the current implementation.
- T0.6: the OS test matrix runs uncached deterministic tests with provider API
  keys blanked; no paid provider calls are enabled by this workflow.
- T0.4: synthetic single-module, monorepo, and Windows-workspace development
  fixtures are inventoried separately from the selected Mili validation commit.
  A focused clean-commit smoke baseline records 8 passing tests. A PostgreSQL
  18.3 probe found the MLI-02 cursor precision defect; comparative agent-task
  baselines remain part of T9.2.
- T1.1–T1.5: generation preserves skills and structured workspaces; generated
  files use ownership hashes; approval records bind rule/evidence fingerprints;
  discovery rejects conflicting IDs; drift reports evidence presence and
  coverage separately from conformance.
- T1.6: `testdata/clones/without-forge/` contains Codex and Claude Code clones
  with static native instructions and no HarnessForge configuration.
- T1.7: deterministic repository-level tests for generation, ownership, review,
  discovery, and drift pass. `TestWithoutForgeClonesPreserveExportedContract`
  now checks both static consumer clones for absence of Forge state and verifies
  exported rules, skill references/content, gates, and project manifests.
  `go test -count=1 ./internal/generation/... ./internal/harness/... ./internal/discovery/... ./internal/drift/... ./cmd/harnessforge` passed after this change. External clone-agent acceptance remains unverified; see
  [P0–T1 acceptance](acceptance/p0-t1.md).
- T2.1: a shared layout resolver discovers `.harness` or `.forge`, rejects
  ambiguous coexistence unless selection is explicit, rejects symlinked
  layout directories and referenced path components, and resolves safe
  repository-relative paths. Regression tests cover internal and external
  symlink targets for both layout directories and references.
- T2.2–T2.6: `.forge/forge.yaml` has a versioned contract independent from
  Harness IR v1/v2; offline loading preserves the complete manifest, and the
  legacy compatibility projection carries project, architecture, skills, and
  gates without aliasing mutable fields. `LoadProject` also loads the referenced
  typed knowledge documents with bounded safe reads, validates their schemas
  and IDs, and preserves review, health, evidence, and provenance separately
  from legacy rules.
  Forge-to-legacy generation now fails closed with directions to native sync,
  rather than emitting a partial Harness. Versioned migration round-trip
  fixtures preserve `architecture.styles` and the legacy skill approval status
  in the Forge manifest. `TestPreviewToForgePreservesEveryMappedLegacyField`
  compares all current legacy fields with their manifest/knowledge mappings;
  semantic review changes (legacy approved rules become Forge candidates) are
  explicit and retain the original review metadata.
- T3.1–T3.4: migration preview reports unmapped choices, apply preserves the
  legacy source, rollback protects modified/unowned output, and v1 migration
  remains compatible. Applying a Forge migration now requires the SHA-256
  digest printed by the reviewed preview; changed sources or choices invalidate
  it. Skill status and evidence now survive conversion through the versioned
  manifest and schema. Cross-layout legacy command compatibility is explicit:
  legacy `generate` directs Forge projects to native `sync` rather than emitting
  a partial document. Explicit `review --layout forge` also fails closed when
  Forge is missing and cannot fall through to mutate a Harness file.
- T4.1–T4.6: deterministic Codex/Claude compile adapters, shared ownership
  manifest, architecture metadata in both native exports, dry-run/check/apply,
  collision and edit protection, staging, clone
  ownership, idempotence, and symlink checks are implemented. T4.5 now records a
  durable recovery journal before publishing files; interrupted transactions
  roll back on the next apply, and post-interruption human edits are preserved
  with an explicit conflict. Concurrent `sync` writers now serialize through
  an OS-backed repository lock stored in the user cache; lock waits honor
  cancellation, and dry-run remains read-only. Concurrent-apply, cross-process
  lock, and lock-cancellation tests pass on Windows; native target scope semantics remain
  unverified. Dry-run reports unmanaged, edited, and unsafe output conflicts
  without writing; `sync --check` lists missing current outputs. Apply captures
  preflight snapshots and rechecks outputs immediately before removal and
  publication. A deterministic concurrent-create test confirms that human
  bytes survive and earlier transaction outputs are restored.
- T5.1/T5.6: read-only aggregate `check`, static Forge/Harness output drift,
  explicit gate status (`not_run`/blocked/executed), optional gate execution,
  strict `--require-gates`, stale/missing approved knowledge failure, and CI
  drift workflow are implemented.
  `doctor` discovers either project layout, validates Forge references, and
  reports declared policies and gates without claiming to enforce or run them.
  Harness diagnostics remain supported.
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
  Counter failures and negative values declare the byte-estimator fallback in
  the result instead of silently reporting the requested counter; the entire
  selection is recomputed in bytes after an intermittent failure so values
  from incompatible estimators are never mixed.
  T6.4 adds `context explain --compare-knowledge`, which emits paired plans for
  the same prompt, model, estimator, budget, ranking options, and task paths,
  with and without approved knowledge. It reports selected knowledge IDs and
  estimated token delta while labeling the result as a retrieval proxy;
  task-level quality remains unmeasured.
- T7.1–T7.5: local observation capture, review, candidate publication,
  audit provenance, and checkout isolation are implemented and covered. T7.3
  memory publication reuses an existing candidate/approved item when content, kind, scope,
  keywords, and evidence match, without changing its provenance or review;
  rejected/deprecated items do not block a new candidate. T7.4 supports
  preview/apply, age and quota retention, pending-item
  preservation, atomic snapshot replacement, and safe retry after an injected
  interruption; pending items survive and repeated collection is idempotent.
  Capture, review, and GC serialize cross-process read-modify-write operations
  with OS file locks so concurrent mutations do not lose observations or reviews.
- T8.1: the official-doc behavior matrix exists; Cursor release/runtime
  acceptance remains open. T8.2: a local-mock runtime check on Windows with
  `@opencode/cli@2.0.18` confirms the effective system context for global and
  project-root `AGENTS.md`, ancestor inclusion from `packages/api`, and
  `OPENCODE_DISABLE_PROJECT_CONFIG=1` retaining only global instructions.
  A separate root-started `read` call confirms nested rules are appended to
  the next request after the agent reads inside `packages/api`. V1 behavior,
  other boundary cases, and actual Forge adapter publication remain unverified. See
  [OpenCode adapter validation](OPENCODE-ADAPTER-VALIDATION.md).
- T8.3: the read-only context resolver is exposed through MCP JSON-RPC stdio
  using the legacy initialize lifecycle. It negotiates `2025-11-25` and
  `2025-06-18`, counter-offers `2025-11-25` for unsupported versions, and
  supports resources/list, resources/read, a task-prompt resource template,
  optional repository-relative task paths, bounded messages, and no tools or
  write operations. Protocol behavior is covered locally; remote transport,
  authentication, and client-specific discovery remain out of scope. See
  [MCP context contract](MCP-CONTEXT.md).
- T8.4: sync validates and publishes portable Agent Skills bundles under
  `.agents/skills/` and `.claude/skills/`; generated instruction files link to
  skills without duplicating their bodies. Updates and stale-file cleanup use
  generated-manifest hashes, and symlinks/special files are rejected.
- T8.5: the opt-in gate runner supports workspace, timeout, cancellation and
  bounded output. Cancellation terminates the shell process tree on supported
  operating systems. Gates inherit the HarnessForge process environment and
  may override named variables; the secret-handling limit is documented in
  [quality-gate guidance](QUALITY-GATES.md). Policies marked `enforced` fail
  closed because no policy enforcement executor is implemented. T8.6 adds a
  dedicated CI workflow that
  verifies Forge sync and runs the repository's `go test ./...` gate; both
  commands passed locally. The workflow has not yet run on GitHub.
- T9.1: the user approved MLI-01–05, Codex CLI local defaults, and three
  repetitions. The pinned Mili commit, four-file team bundle, generated Forge
  source/export, task text, model/effort, run rotation, and baseline smoke are
  hash-recorded in the pilot freeze files. The original Mili checkout is intact.
  All five independent task evaluators are frozen by hash. The MLI-02 evaluator
  uses the real service/controller and its three characterization tests pass
  on Temurin 25.0.4.1. The four malformed cursor observations are 500, 200,
  200, and 500; the accepted malformed cursors cause repository queries.
  Baseline probes found cross-tenant exposure in MLI-01, stale ACTIVE cache and accepted ingest
  after pause/delete in MLI-03, and missing-event HTTP 404 failures in MLI-04;
  MLI-04 FAILED/204 and non-FAILED/409 controls pass. The frozen PostgreSQL
  18.3 probe traversed tied millisecond rows without loss but skipped 70 of 120 rows when timestamps had
  submillisecond precision: the API cursor truncates to milliseconds. The
  pinned MLI-02 baseline therefore fails pagination completeness. The MLI-05
  JsonPath probe confirms the frozen malformed expression is rejected. These
  evaluator checks are not coding-agent pilot runs. The user-approved MLI-01
  repetition-1 runs are complete on three isolated clones: baseline, team, and
  Forge each passed 3/3 independent v3 acceptance checks. The agent-authored
  focused test suites were not run according to retained CLI reports; earlier
  contradictory summaries claiming success are superseded. Diff sizes were
  measured, while duration, tokens, review quality, rule violations and
  instruction-source discovery were not captured. Repetition 2 has one valid
  Team attempt: its independent evaluator passed 3/3 and later focused
  verification passed 12/12; the Team task changed 9 source/test files (+75/-29
  tracked lines). Baseline and Forge reused clones across multiple
  agent sessions and are invalid; a diagnostic Forge build on that clone failed
  at `InboundEventRepository.java:76`. A fresh Forge retry also could not run:
  the CLI executor remained read-only despite the requested workspace-write
  setting. The repetition remains incomplete and is not comparative. See [the database result](pilot/mili-postgres-keyset-v1.json),
  [partial pilot results](pilot/mili-results-v1.md), and [repetition 2 record](pilot/mli01-repetition-2.md).
  T9.1 protocol is versioned. T9.2 has four valid condition runs across one
  complete task/repetition plus a single Team attempt; T9.3 records available
  measures and missing-data limits. T9.5
  publishes descriptive partial outcomes only. T9.6 provisionally defers a
  custom runtime pending the complete pilot and T7/T8 runtime evidence; see
  [ADR 0002](adr/0002-runtime-executor-decision.md). The remaining 41 planned
  condition runs are outstanding, so neither T9.5 nor T9.6 has final acceptance.

## Verification

Passed:

```text
go test ./...
go vet ./...
go run ./cmd/harnessforge sync --repository . --check
go run ./cmd/harnessforge check --repository . --layout forge --run-gates --format json
```

The local full test suite, static analysis, sync drift check, and declared Go
gate pass on Windows. The cross-platform CI matrix has not yet been run. No
direct provider API calls were made; the approved Codex CLI was used for the
Mili pilot attempts recorded above.

## Remaining validation

- Cursor, broad OpenCode compatibility, and external Codex/Claude runtime
  validation, end-to-end clone agent runs, pilot measurements, and the runtime
  decision require exact agent builds and representative repositories; the
  narrowly scoped OpenCode V2.0.18 local-mock evidence is recorded separately
  and is not generalized to other builds or adapter behavior.
- T9.2–T9.5 need completion of the frozen run matrix and outcome measurements.
  Mili's clean HEAD, approved tasks, run configuration, instruction bundles,
  and PostgreSQL keyset finding are versioned; the original modified working
  tree is excluded. Only one complete task/repetition exists so far, plus one
  valid Team condition attempt. Synthetic Go
  and Windows fixtures remain development data, not pilot evidence.
- T6.2 now supports explicit knowledge keywords and task paths through the
  context API and `context explain --task-path`; glob matching is segment-aware
  with `**`, and backend/frontend scope fixtures pass. T6.3 reports early
  prompt-envelope overflow with the same configured estimator and an explicit
  excluded-prompt reason. T6.4's paired token/selection report is a retrieval
  proxy and must not be described as measured task quality.
- T1.7/T2/T3/T4/T5 also retain acceptance gaps listed in their status entries;
  static fixtures and repository-level tests are not external runtime evidence.
- This checkout contains pre-existing repository-context/retrieval changes;
  they remain outside the Forge plan task commits.
