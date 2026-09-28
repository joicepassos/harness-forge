# ADR 0002: Defer a HarnessForge-owned runtime

- Status: deferred, provisional
- Date: 2026-09-28
- Related work: Forge evolution plan T9.6

## Context

The plan conditions a custom runtime on demonstrated problems that existing
agent executors do not solve. The Mili pilot has one valid task/repetition across
its baseline, Team and Forge conditions. The independent MLI-01 evaluator
passed 3/3 checks in each condition. Agent-authored test suites were not run,
repetition 2 was excluded for contamination, and duration, token use, review
quality, instruction-rule violations and discovery were not measured. This is
not enough comparative evidence to justify building or rejecting a custom
runtime on product grounds.

## Decision

Defer implementation of a HarnessForge-owned agent runtime. Continue to use
existing agent executors for the pilot and invest in versioned configuration,
exports, validation, context selection and verifiable gates. Revisit this ADR
after the approved pilot matrix and T7/T8 runtime integration evidence are
complete.

## Consequences

- No custom scheduler, agent loop, model client or runtime state machine is
  authorized by this decision.
- Runtime deferral is provisional; it is not a measured finding that existing
  executors are sufficient for every workflow.
- A later proposal should identify a concrete unsolved executor problem, report
  user demand and operating cost, and compare against the existing executor
  under reproducible tasks.

## Evidence limits

The current pilot does not meet T9.5's full dataset requirement. The partial
results are recorded in [Mili pilot results](../pilot/mili-results-v1.md), and
the excluded repetition is detailed in [the repetition 2 report](../pilot/mli01-repetition-2.md).
This ADR records a conservative sequencing decision only; it is not a final
runtime evaluation.
