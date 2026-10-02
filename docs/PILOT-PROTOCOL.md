# HarnessForge pilot protocol (T9.1–T9.5)

Status: the approved Mili pilot protocol and task/configuration freeze are
versioned; partial external agent runs are recorded, and no paid provider API
calls were made. The frozen plan selects MLI-01–05, Codex CLI 0.158.0-alpha.2.1
with the approved local defaults, and three repetitions across baseline, Team,
and Forge conditions (45 planned runs). See the [freeze](pilot/mili-condition-freeze-v1.json),
[task approval](pilot/mili-task-approval-v1.json), and [partial results](pilot/mili-results-v1.md).
The [task draft](pilot/tasks-draft-v1.md) is the hash-pinned source for the
approved task texts; only MLI-01–05 are approved for this pilot. The broader
multi-repository and multi-agent design below remains a proposed future study,
not the current pilot scope.

## Design

Use three repositories: a Go service, a Java service, and an application with at
least two independently tested workspaces. Select 12–20 tasks with executable
acceptance criteria, including bug fixes, feature work, and maintenance. Freeze
each repository commit, task text, baseline team instructions, Forge manifest
and exports, agent product/build, model, tool permissions, and evaluation rubric
before collecting results.

Compare three conditions per task: (A) no additional project instructions,
(B) current team instructions, and (C) approved HarnessForge exports. P4 may add
(D) dynamically resolved context, reported as a separate condition. Use at
least two agent products and three repetitions where budget permits. Randomize
condition order per task and do not change the task after observing a result.

## Measurements

For every run, record functional and regression tests, independent human review,
rule violations, duration, measured tokens (or estimator and estimated tokens),
observed cost when available, relevant-source recall, instruction discovery,
and failure/cancellation. Separate missing data from zero. Reviewers should not
see the condition label while rating the code when practical.

Do not claim quality improvement from retrieval terms, context length, or model
self-report. Report per-repository and per-agent outcomes, variation across
repetitions, and deviations. The initial sample is exploratory and does not
establish statistical generalization.

## Run record

Store one JSON object per attempted run with this shape (extend only with
versioned fields). Missing or unmeasured values are `null`, never zero; a
separate status distinguishes valid runs from excluded attempts. Version 2
adds a unique attempt ID, explicit run status, exclusion reason, and nullable
measurements. The
initial ledger contains only evidence already retained in the pilot records;
it does not impute missing values.

```json
{
  "schema_version": 2,
  "attempt_id": "<unique attempt ID>",
  "task_id": "go-01",
  "repository": "go-service",
  "commit": "<immutable git SHA>",
  "condition": "baseline|team|forge|dynamic",
  "status": "valid|excluded",
  "exclusion_reason": null,
  "agent": "<product and build>",
  "model": "<exact model identifier>",
  "repeat": 1,
  "started_at": null,
  "duration_seconds": null,
  "tokens": {"value": null, "method": null},
  "cost": {"value": null, "currency": null},
  "tests": {"status": "measured|not_run|unknown", "passed": null, "failed": null, "commands": null},
  "human_review": {"reviewer": null, "score": null, "notes": null},
  "rule_violations": null,
  "instruction_discovery": "observed|not_observed|unknown",
  "failure": null
}
```

Use an empty array only when collection was performed and found no entries;
use `null` when collection was not performed or the value is unknown.

The backfilled [run ledger](pilot/mili-run-ledger-v2.jsonl) records four valid
condition runs and four excluded repetition-2 attempts, including a fresh
retry blocked while reading project instructions. It intentionally leaves
unrecorded timings, tokens, cost, human review, rule violations and instruction
discovery as `null`/`unknown`. This ledger is
an audit aid, not completion of T9.2/T9.3 or a substitute for the 45-run matrix.

## Release gate and current status

The Mili repository is pinned to clean commit
`49402ad341a7e8a01aee7968d1b10d91b44c0d1c`; the original modified checkout is
excluded. The freeze records the condition overlays and hashes, approved task
IDs and task source, agent/model/tool configuration, run rotation, and evaluator
sources. The one-time baseline smoke passed 8 selected tests, and separate
characterization probes recorded baseline defects, including the MLI-02
PostgreSQL keyset cursor precision issue. These checks establish environment
and evaluator baselines; they are not comparative coding-agent task runs.

Four of 45 planned condition runs are currently valid: MLI-01 repetition 1 has
baseline, Team, and Forge runs, and repetition 2 has one Team attempt. The
independent MLI-01 v3 acceptance evaluator passed 3/3 checks in each valid run;
the repetition 2 Team attempt also passed a later focused verification (12/12).
Repetition 2 has no valid baseline or Forge run because the clones were reused
across agent sessions; a diagnostic Forge build from that contaminated clone
failed, and a fresh retry could not proceed because the CLI executor was
read-only. These excluded attempts are not counted as valid results. Forty-one
planned condition runs remain, so T9.2 is partial and the matrix is incomplete.

An integrity audit on 2026-09-28 found that the tracked MLI-01 rubric changed
after the digest in the freeze was recorded, and the approval-record digest
cannot be reproduced from its tracked history. No further comparison run should
use this freeze until the immutable approval snapshot and rubric are reconciled
and reviewed. See the [freeze integrity audit](pilot/freeze-integrity-audit-2026-09-28.md).

Agent-authored focused tests were not run in repetition 1. Duration, tokens,
review quality, rule violations, and instruction-source discovery were not
captured for the valid runs; these values are missing, not zero. The pilot is
therefore insufficient for T9.3's complete measurement set. No dynamic-context
condition has been run, so T9.4 remains pending. The [partial results](pilot/mili-results-v1.md)
are descriptive only and do not establish a quality or efficiency advantage;
T9.5 remains incomplete. The [runtime decision ADR](adr/0002-runtime-executor-decision.md)
provisionally defers a custom runtime until the pilot and T7/T8 runtime evidence
are complete; it is not a final product conclusion (T9.6).

The broader proposed validation design calls for multiple repositories and
agent products. The approved Mili pilot is narrower and exploratory, so its
results must not be generalized to those populations. The inventory's other
repositories and local feasibility checks are not included in the frozen Mili
comparison.
