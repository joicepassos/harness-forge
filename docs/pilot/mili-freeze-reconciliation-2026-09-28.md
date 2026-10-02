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

## Follow-up approval and evaluator revision

On 2026-09-28, the user confirmed: “Sim: MLI-01–05, Codex CLI padrão, 3
repetições” in response to using the task texts from `tasks-draft-v1.md` and
the default Codex CLI configuration. The approved MLI-01 task says foreign
detail and reprocess must be indistinguishable from not found. This resolves
the acceptance choice in favor of the draft task text.

Rubric v2 makes that observable requirement explicit: foreign and unknown
detail/reprocess requests both return 404 with equal content type and response
body; foreign-event responses contain no tenant-B event ID; and foreign
reprocess never mutates the event. `mili-evaluator-v2.md` and
`mili-evaluator-sources-v2/Mli01AcceptanceTestV4.java` record this revision.
The earlier V3 evaluator allowed 400 or 404 and did not compare detail
responses, so its MLI-01 outcomes do not establish the approved criterion.
Those results remain historical and must be re-evaluated with V4 or rerun from
fresh clones before they count toward the pilot.

The immutable historical approval snapshot and v1 freeze remain unchanged.
`mili-condition-freeze-v2.json` records the approved task/configuration and the
rubric/evaluator hashes. The V4 evaluator compiled and ran against a fresh clone
of the pinned baseline (3 tests, 3 expected contract failures). No scored agent
condition has run under revision 2 yet; prior V3 outcomes are not included in
its 0/45 count. The approved task set, Codex CLI defaults, and three
repetitions are unchanged.
