# MLI-01 repetition 2

Status: excluded from comparative scoring. Intended condition rotation was
team, Forge, baseline. Three attempted condition runs are retained locally
under the ignored `.pilot-runs/` directory for audit and are not counted as
valid runs in the 45-run plan.

## Exclusion evidence

- The purported clean baseline clone contains task-driven changes to production
  files and tests, so it did not run the frozen baseline condition.
- The team clone's independent MLI-01 v3 acceptance evaluator passed 3/3 checks,
  but the agent-authored focused tests reported 1 failure among 8 tests.
- Forge did not compile: `InboundEventRepository.java` reported a missing
  return statement. A second Forge agent process also ran, contaminating the
  one-agent condition.
- The repeated baseline evaluator attempt did not produce a comparable result.

These observations are retained as execution diagnostics only. No comparative
claim or repetition-2 pass is made. The original Mili checkout remained intact.
