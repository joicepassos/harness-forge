# MLI-01 repetition 1 attempt under freeze v2

Date: 2026-09-28. Freeze: `mili-condition-freeze-v2.json`, SHA-256
`229545a0aa9494ed9102f9842943b0954c8c2a0c773b29b3053ede71249566c6`.

## Outcome

No valid agent run was completed. The baseline, Team, and Forge runs each used
a fresh clone at commit
`49402ad341a7e8a01aee7968d1b10d91b44c0d1c`; the two overlays matched the
frozen four-file hashes. The Codex CLI sessions exited with code 0, but each
reported that the host environment policy blocked its PowerShell/repository
commands before it could inspect or change the task checkout. Each clone
remained without task implementation changes. A successful CLI process exit
alone is not evidence that the task ran.

The evaluator source was applied separately to the three untouched clones.
All three runs failed the same three MLI-01 assertions: tenant-scoped list,
foreign detail equivalent to not found, and foreign reprocess equivalent to
not found without mutation. These checks reproduce the pinned baseline defects
under each instruction overlay; they are not condition outcomes and count as
zero of the 45 planned agent runs.

| Condition | CLI session | Independent V4 evaluator | Interpretation |
| --- | --- | --- | --- |
| Baseline | Blocked before repository access; exit 0, 18.702 s | 3 tests, 3 failures; exit 1, 127.707 s | Baseline characterization only |
| Team | Blocked before task changes; exit 0, 13.923 s | 3 tests, 3 failures; exit 1, 199.704 s | Overlay characterization only |
| Forge | Blocked before task changes; exit 0, 17.857 s | 3 tests, 3 failures; exit 1, 91.96 s | Overlay characterization only |

The frozen rubric v2 command names `Mli01AcceptanceTest`, while its pinned
source declares `Mli01AcceptanceTestV4`. The Team and Forge evaluator attempts
used the actual frozen class name; Forge also recorded the initial no-match
filter attempt before rerunning the correct class. This is a command-name
erratum only. The freeze and source hashes were not changed.

## Evidence

Raw local logs are under `.pilot-runs/` and were not added to the repository:

- Baseline CLI and evaluator: `mli01-v2-r1-baseline-agent.log` and
  `mli01-v2-r1-baseline-evaluator-20260928.log`.
- Team CLI and evaluator: `mli01-v2-r1-team-agent-20260928.jsonl` and
  `mli01-v2-r1-team-evaluator-20260928.log`.
- Forge record: `logs/mli01-v2-r1-forge-record-20260928-193523-394.json`.

To resume comparable runs, the frozen Codex CLI execution policy must be able
to run the agent's repository commands with workspace-write access. Changing
the sandbox or approval mode would change the frozen condition and requires a
new reviewed freeze before those runs can count.
