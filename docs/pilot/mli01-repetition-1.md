# MLI-01 repetition 1

User-approved pilot configuration: Mili commit `49402ad341a7e8a01aee7968d1b10d91b44c0d1c`; task MLI-01; Codex CLI `0.158.0-alpha.2.1`, model `gpt-6-luna`, effort `low`; Temurin 25.0.4.1; three repetitions planned overall. The original Mili checkout was not modified.

## Results

| Condition | Agent-authored focused tests | Independent MLI-01 v3 evaluator | Outcome |
| --- | --- | ---: | --- |
| Baseline | Not run; the CLI report said Java was unavailable. | 3/3 passed | Acceptance pass; compile/test evidence for agent changes incomplete |
| Team | Not run; the CLI report said Java 21 was available but Java 25 was required. | 3/3 passed | Acceptance pass; compile/test evidence for agent changes incomplete |
| Forge | Not run; the CLI report said Java 21 was available but Java 25 was required. | 3/3 passed | Acceptance pass; compile/test evidence for agent changes incomplete |

The evaluator ran from each pinned clone with the condition overlay and the exact frozen source hash, using Gradle 9.5.1 and Eclipse Temurin 25.0.4.1+1-LTS on Windows 11. It checked tenant-A listing despite a forged tenant query, detail isolation and tenant-A visibility, authenticated tenant arguments, equivalent not-found status for foreign and unknown reprocess IDs, and no reset for the foreign ID. This independent evaluator passed in all three conditions. The agent CLI reports did not execute the focused tests due to missing or incompatible Java; contradictory historical summaries claiming focused-test success are superseded by the retained run reports. Therefore agent-authored test suites and full project regressions were not verified for repetition 1.

Diff sizes from the retained clean-clone Git diffs were baseline 6 files / 37 insertions / 17 deletions, team 6 files / 60 insertions / 35 deletions, and Forge 7 files / 70 insertions / 32 deletions (excluding the untracked Forge controller test and exception files). Duration, token consumption, code-review scores, rule violations, and agent-discovery behavior were not captured. They are missing data, not zero.

## Measurement corrections

The frozen MLI-01 v1 source expected only the old repository method name; v2 switched to a new method name and over-constrained response bodies. Both versions caused harness-only failures and are excluded from scoring. MLI-01 v3 accepts both production API names, checks tenant arguments, requires a 404 for detail, accepts equivalent 400/404 not-found status for reprocess, and does not reject the caller-supplied event ID in error bodies. Its SHA-256 is `2c3d4648c2c0316eafe46b0f946ecbb45f2128cf2b169aaffa0001b6519e6a88`. All three conditions were rerun with this exact source.

Each run is a fresh detached clone under the ignored `.pilot-runs/` directory. Agent diffs and CLI reports are retained locally for audit and excluded from Git. `mili-condition-freeze-v1.json` tracks 42 valid agent runs still outstanding; the three repetition-2 attempts are excluded. This single task/repetition does not establish a comparative quality advantage.
