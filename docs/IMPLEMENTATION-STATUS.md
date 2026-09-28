# Implementation status

Date: 2026-09-27. Branch: `codex/forge-evolution-mvp`. Base: `bb226d8`.

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
  ambiguous coexistence unless selection is explicit, and resolves safe
  repository-relative paths.
- T2.2–T2.6: `.forge/forge.yaml` has a versioned contract independent from
  Harness IR v1/v2; offline loading and reference validation are covered.
  Forge-to-legacy generation now fails closed with directions to native sync,
  rather than emitting a partial Harness. Versioned migration round-trip
  fixtures preserve `architecture.styles` in the Forge manifest; broader
  field-level parity beyond explicitly mapped fields remains open.
- T3.1–T3.4: migration preview reports unmapped choices, apply preserves the
  legacy source, rollback protects modified/unowned output, and v1 migration
  remains compatible. Skill evidence now survives conversion through the
  versioned manifest and schema. Cross-layout legacy command compatibility is
  explicit: legacy `generate` directs Forge projects to native `sync` rather
  than emitting a partial document.
- T4.1–T4.6: deterministic Codex/Claude compile adapters, shared ownership
  manifest, architecture metadata in both native exports, dry-run/check/apply,
  collision and edit protection, staging, clone
  ownership, idempotence, and symlink checks are implemented. T4.5 now records a
  durable recovery journal before publishing files; interrupted transactions
  roll back on the next apply, and post-interruption human edits are preserved
  with an explicit conflict. Native target scope semantics and concurrent
  writers remain unverified.
- T5.1/T5.6: read-only aggregate `check`, static Forge/Harness output drift,
  explicit gate status (`not_run`/blocked/executed), optional gate execution,
  strict `--require-gates`, stale/missing approved knowledge failure, and CI
  drift workflow are implemented.
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
  instruction-source discovery were not captured. Repetition 2 is explicitly
  excluded because the purported baseline clone was modified, a Team focused
  test failed, Forge did not compile, and Forge had a duplicate agent run. See
  [the database result](pilot/mili-postgres-keyset-v1.json), [partial pilot
  results](pilot/mili-results-v1.md), and the [repetition 2 exclusion](pilot/mli01-repetition-2.md).
  T9.1 protocol is versioned. T9.2 has one valid task/repetition with three
  conditions; T9.3 records available measures and missing-data limits. T9.5
  publishes descriptive partial outcomes only. T9.6 provisionally defers a
  custom runtime pending the complete pilot and T7/T8 runtime evidence; see
  [ADR 0002](adr/0002-runtime-executor-decision.md). The remaining 42 valid
  agent runs are outstanding, so neither T9.5 nor T9.6 has final acceptance.

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
paid provider calls were made.

## Remaining validation

- External Cursor/OpenCode/Codex/Claude runtime validation, end-to-end clone
  agent runs, pilot measurements, and the runtime decision require pinned agent
  builds and representative repositories; none are fabricated here.
- T9.2–T9.5 need completion of the frozen run matrix and outcome measurements.
  Mili's clean HEAD, approved tasks, run configuration, instruction bundles,
  and PostgreSQL keyset finding are versioned; the original modified working
  tree is excluded. Only one valid task/repetition exists so far. Synthetic Go
  and Windows fixtures remain development data, not pilot evidence.
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
