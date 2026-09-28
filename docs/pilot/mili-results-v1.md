# HarnessForge Mili pilot results (partial)

Status: incomplete; descriptive evidence only. The user-approved plan calls for
five tasks, three repetitions, and three conditions (45 agent runs total).
Only MLI-01 repetition 1 has three valid condition runs. Repetition 2 was
excluded after condition contamination; the other 42 planned valid runs have
not been executed. No runtime claim is based on this partial dataset.

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

## Repetition 2 exclusion

Repetition 2 produced no valid comparative result. The nominal baseline clone
contained production and test changes for the task; the Team clone had one
failure in its eight agent-authored focused tests; Forge failed Java compilation
because of a missing return and had a duplicate agent process. Detailed evidence
is in [the repetition 2 log](mli01-repetition-2.md). The runs remain excluded
from scoring.

## Limits and follow-up

One task with one valid repetition is far below the frozen sample plan and
cannot support a quality or efficiency advantage for Team or Forge. The
Mili-specific continuation is to rerun the contaminated rotation from newly
verified clean clones, finish repetitions 2 and 3, execute the remaining four
tasks, and capture runtime, token use, full project-test results, review
outcomes, rule violations, and instruction-source discovery for each run.
