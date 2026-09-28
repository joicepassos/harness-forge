# MLI-01 repetition 2

Status: incomplete repetition; one valid Team condition attempt, with baseline
and Forge condition attempts invalidated by multiple agent sessions on the same
clone. Logs and clones are retained locally under the ignored `.pilot-runs/`
directory. A valid Team run is retained in the 45-run plan, but no complete
three-condition comparison can be made from this repetition.

## Exclusion evidence

- Baseline used two Codex sessions against one clone. The first was read-only and
  inspected the task; the second implemented it. Code changes are expected task
  output, but reusing the clone after the first task session violates the frozen
  fresh-clone/session isolation, so baseline is invalidated.
- Team used one Codex task session and the frozen overlay. Its independent MLI-01
  v3 evaluator passed 3/3 checks. A post-run focused verification of the existing
  webhook service/domain/cache suites plus the independent evaluator passed
  12/12 tests. The agent did not run Gradle during its task session because the
  Java 25 toolchain was not configured then.
- Forge used two Codex sessions in one clone; the second inspected and continued
  after task changes existed, so Forge is invalidated for clone/session reuse.
  A later diagnostic compilation in that clone failed at
  `InboundEventRepository.java:76` with a missing return statement. This is a
  code outcome for the contaminated clone, not a valid comparative Forge run.
- A fresh Forge retry was prepared from the pinned commit with the frozen
  overlay, but the Codex CLI session reported its executor as read-only and
  could not inspect or modify the clone despite the requested `workspace-write`
  sandbox. No task changes or tests occurred, so this retry is also invalid.

The Team attempt is counted as one valid condition run, not as a complete
repetition. No comparative claim or repetition-2 pass is made. The original
Mili checkout remained intact.
