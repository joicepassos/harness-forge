# MLI-01 baseline retry blocked under freeze v2

Date: 2026-09-28 (America/Sao_Paulo). Freeze:
`mili-condition-freeze-v2.json`.

## Attempt

- Condition: baseline instructions, MLI-01, repetition 1.
- Checkout: a new detached clone at
  `49402ad341a7e8a01aee7968d1b10d91b44c0d1c`; `git status --porcelain` was
  empty before execution and remained empty afterward.
- Working directory: `backend/core`.
- Agent: Codex CLI `0.158.0-alpha.2.1`, `gpt-6-luna`, low reasoning, ephemeral
  session, `workspace-write`, approval policy `never`; user configuration was
  ignored as required by the freeze.
- Prompt: the approved MLI-01 text in `tasks-draft-v1.md`.

## Outcome

The CLI process exited 0, but its first repository search command was rejected
by the host workspace policy. A second attempt to list the checkout was also
rejected. The agent stopped without reading or changing repository files. No
independent evaluator was run. This is an invalid agent run and contributes
zero to the frozen 0/45 score.

The CLI session reported 76,413 input tokens (60,416 cached) and 398 output
tokens. The session used ChatGPT sign-in and the plan's included Codex usage;
no API key, purchased credits, or separate provider was configured. No further
retry should be made under the same host policy because it blocks repository
access before the task can start.

This attempt confirms the execution-policy blocker; it is not evidence about
the baseline implementation or any instruction condition.
