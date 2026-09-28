# Mili pilot freeze reconciliation

Date: 2026-09-28. No new pilot condition or database operation was run.

## Recoverable user approval

`mili-task-approval-v2-snapshot.json` is a byte-for-byte copy of
`docs/pilot/mili-task-approval-v1.json` from Git commit `df787e2a`. Its SHA-256
is `78b3923414cc008d8746478b79b8d1ff8e92b9e8b5e37805bfa2e023fb51a9c2`; the
source Git blob is `7779acc2032a9de938218e01aff7ceaa20504a1b`.

The recovered record confirms MLI-01–05, the pinned Mili commit, Codex CLI
0.158.0-alpha.2.1 with local defaults (`gpt-6-luna`, low effort, default tier),
and three repetitions per condition. It is marked `partially_frozen` and
`run_performed: false`; it does not approve the later rubric changes or complete
the evaluator/condition freeze. The mutable approval record and the v1 freeze
remain unchanged.

## Acceptance decision still required

The rubric at the recorded freeze digest differs from the current rubric. For
MLI-01 detail lookup, the original criterion requires the foreign event to have
the same externally observable not-found response as an unknown ID. The current
wording requires HTTP 404 and no tenant-B disclosure. The user must select which
criterion governs the pilot.

The reprocess criterion also needs an exact status decision. The original says
the foreign event returns the same 404 as an unknown ID; the current wording
says the same not-found status. The MLI-01 v3 evaluator permits 400 or 404 for
each response independently and does not require them to match. Its detail
assertion also only checks that the body omits the tenant-A event ID; it does
not establish 404, response equivalence, or absence of the tenant-B event ID.
Thus prior v3 results do not establish either rubric version in full.

Before more comparison runs, record the selected detail and reprocess criteria,
write a reviewed evaluator that asserts them, and freeze a new rubric/evaluator
pair with reproducible hashes. Existing run records remain historical until
their evidence can be evaluated against that pair.
