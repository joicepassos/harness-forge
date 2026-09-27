# HarnessForge pilot protocol (T9.1–T9.5)

Status: protocol and data format only; no external agent runs or paid provider
calls are represented here. Results must be added only after a run is completed.

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

Store one JSON object per run with this shape (extend only with versioned fields):

```json
{
  "schema_version": 1,
  "task_id": "go-01",
  "repository": "go-service",
  "commit": "<immutable git SHA>",
  "condition": "baseline|team|forge|dynamic",
  "agent": "<product and build>",
  "model": "<exact model identifier>",
  "repeat": 1,
  "started_at": "<UTC timestamp>",
  "duration_seconds": 0,
  "tokens": {"value": 0, "method": "reported|provider-counter|byte-upper-bound"},
  "cost": {"value": null, "currency": null},
  "tests": {"passed": 0, "failed": 0, "commands": []},
  "human_review": {"reviewer": "<blinded ID>", "score": null, "notes": ""},
  "rule_violations": [],
  "instruction_discovery": "observed|not_observed|unknown",
  "failure": null
}
```

## Release gate and current status

Before the pilot, populate the task and repository inventory, pin software
versions, and validate condition isolation. No run has been performed by this
implementation; metric targets must be set after the baseline is collected and
before comparing Forge results. T9.6 (runtime decision) remains deferred until
P5–P7 and the pilot evidence exist. No runtime recommendation can be inferred
from an empty dataset.
