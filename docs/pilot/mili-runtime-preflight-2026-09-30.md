# Mili v2 runtime preflight (no agent run)

Date: 2026-09-30 (America/Sao_Paulo). The governing protocol is
`mili-condition-freeze-v2.json`; this check did not change it or execute a
pilot condition.

The freeze pins Codex CLI `0.158.0-alpha.2.1`, `gpt-6-luna` with low reasoning,
`workspace-write`, approval `never`, ignored user configuration, an ephemeral
session, and a fresh detached clone of Mili commit
`49402ad341a7e8a01aee7968d1b10d91b44c0d1c` per condition run.

`codex --version` currently reports `codex-cli 0.159.0`. The only `codex.exe`
found under the app's local binary directory is the one on `PATH`; no pinned
`0.158.0-alpha.2.1` executable was found there. The earlier v2 attempts
recorded in [the MLI-01 attempt](mili-mli01-r1-attempt-v2.md) and
[blocked retry](mili-mli01-v2-r1-baseline-blocked-20260928.md) were denied
repository commands by the host policy before any task work. There is no
evidence that this nested CLI policy has changed. This preflight did not retry
the agent, consume model usage, or alter the 0/45 valid-run count.

The next comparable run requires the pinned CLI build, a fresh clean clone,
and effective repository command access under the frozen execution settings.
Using CLI `0.159.0` or changing those settings requires a separately reviewed
protocol freeze; neither change can be counted under v2 as it stands.
