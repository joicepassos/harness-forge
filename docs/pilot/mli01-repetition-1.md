# MLI-01 repetition 1

User-approved pilot configuration: Mili commit `49402ad341a7e8a01aee7968d1b10d91b44c0d1c`; task MLI-01; Codex CLI `0.158.0-alpha.2.1`, model `gpt-6-luna`, effort `low`; Temurin 25.0.4.1; three repetitions planned overall. The original Mili checkout was not modified.

## Results

| Condition | Agent-authored focused tests | Independent MLI-01 v3 evaluator | Outcome |
| --- | ---: | ---: | --- |
| Baseline | 2/2 passed | 3/3 passed | Pass |
| Team | 2/2 passed | 3/3 passed | Pass |
| Forge | 2/2 passed | 3/3 passed | Pass |

The evaluator ran from each clean pinned clone with the condition overlay, via `gradlew.bat test --tests com.mili.core.webhook.pilot.Mli01AcceptanceTestV3`. It checked tenant-A listing despite a forged tenant query, detail isolation and tenant-A visibility, authenticated tenant arguments, equivalent not-found status for foreign and unknown reprocess IDs, and no reset for the foreign ID. Runtime: Windows 11, Gradle 9.5.1, Eclipse Temurin 25.0.4.1+1-LTS. The task agent sessions had no runtime tokens available inside their environment; focused Gradle checks were run afterward by the evaluator on the unchanged agent diffs.

## Measurement corrections

The frozen MLI-01 v1 source expected only the old repository method name; v2 switched to a new method name and over-constrained response bodies. Both versions caused harness-only failures and are excluded from scoring. MLI-01 v3 accepts both production API names, checks tenant arguments, requires a 404 for detail, accepts equivalent 400/404 not-found status for reprocess, and does not reject the caller-supplied event ID in error bodies. Its SHA-256 is `2c3d4648c2c0316eafe46b0f946ecbb45f2128cf2b169aaffa0001b6519e6a88`. All three conditions were rerun with this exact source.

Each run is a fresh detached clone under the ignored `.pilot-runs/` directory. Agent diffs and full logs are retained locally for audit and excluded from Git. `mili-condition-freeze-v1.json` tracks the 42 outstanding runs. This single task/repetition does not establish a comparative quality advantage.
