# HarnessForge Mili pilot results (partial)

Status: incomplete; descriptive evidence only. The user-approved plan calls for
five tasks, three repetitions, and three conditions (45 agent runs total).
MLI-01 repetition 1 has three valid condition runs. Repetition 2 has one valid
Team condition attempt; its baseline and Forge attempts used multiple agent
sessions on the same clone and are invalid. Four of the 45 planned condition
runs are valid; 41 remain. No runtime claim is based on this partial dataset.

## MLI-01 repetition 1

| Condition | Independent acceptance evaluator | Agent focused suite | Changed files | Tracked insertions/deletions |
| --- | ---: | --- | ---: | ---: |
| Baseline instructions | 3/3 pass | Not run | 8 | +37 / -17 |
| Team instructions | 3/3 pass | Not run | 9 | +60 / -35 |
| Forge exports | 3/3 pass | Not run | 11 | +70 / -32 |

The same frozen MLI-01 v3 evaluator was used in all three conditions. It checks
tenant-isolated listing, detail and reprocess behavior, including authenticated
tenant arguments and no cross-tenant reset. Passing this independent evaluator
does not establish that the agent's complete change compiles or that project
regressions pass. The CLI run reports said focused suites could not run because
of unavailable/incompatible Java; agent-authored suites are therefore recorded
as not run.

Duration, tokens, review quality, instruction-rule violations, and whether the
agent discovered each instruction source were not captured. Those measures are
missing, not zero. The file counts include untracked files in each local clone;
insertions/deletions count tracked files only.

## Repetition 2 status

Repetition 2 is incomplete and has no three-condition comparative result. Its
Team task attempt is valid: the independent evaluator passed 3/3 and a later
focused verification passed 12/12. Baseline and Forge are invalid due to clone
reuse across multiple agent sessions. A diagnostic build of the contaminated
Forge clone failed compilation at `InboundEventRepository.java:76`; that result
does not count as a valid Forge condition run. A fresh Forge retry also stopped
before inspection because the CLI executor was effectively read-only. Detailed evidence is in [the
repetition 2 log](mli01-repetition-2.md).

| Condition | Independent acceptance evaluator | Post-run focused verification | Changed files | Tracked insertions/deletions |
| --- | ---: | ---: | ---: | ---: |
| Team instructions | 3/3 pass | 12/12 pass | 9 | +75 / -29 |

The Team measurements exclude frozen overlay/evaluator files and diagnostic
logs from the task diff. Duration, tokens, review quality, rule violations, and
instruction-source discovery were not captured.

## Limits and follow-up

One task with one valid repetition is far below the frozen sample plan and
cannot support a quality or efficiency advantage for Team or Forge. The
Mili-specific continuation is to rerun invalid conditions from newly verified
clean clones, finish repetitions 2 and 3, execute the remaining four
tasks, and capture runtime, token use, full project-test results, review
outcomes, rule violations, and instruction-source discovery for each run.
