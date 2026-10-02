# Frozen pilot artifact integrity audit

Date: 2026-09-28. This audit is read-only with respect to the approved task,
rubric, condition overlays, and Mili repositories. No pilot agent run or database
operation was performed.

## Findings

The current `mili-condition-freeze-v1.json` records these digests:

| Artifact | Digest in freeze | Current tracked Git blob | Result |
| --- | --- | --- | --- |
| `mili-task-approval-v1.json` | `77f69ad649e2e6eac8c55b8dbde18fb2cb0f81e6c741516c04299a7d029e70d0` | `a6aad341e7f3c642ae7720fffbabd4bb41a78e688d64645fec5944e97132c51a` | Mismatch |
| `mili-evaluator-v1.md` | `eb0212c488b7647c61b537100c08021cb366f4d6e204d26c7518c2acf11d8049` | `a81e0fb319bde08c4c2aa8cb105a78d0d7d380644e7d6dfe60d6cdabd6bafc9c` | Mismatch |

The frozen rubric digest matches the rubric blob at commit `ff0d4bda`.
Commit `a9f24e42` changed the rubric wording for MLI-01 detail and reprocess
responses while leaving the freeze digest unchanged. In particular, the detail
criterion no longer requires the same externally observable not-found response
as an unknown ID. This is a post-freeze acceptance change; the rubric and freeze
must be reconciled through review before additional runs.

The approval record is also a mutable status document: later commits changed
its run-status fields. The current freeze digest does not match the current
approval blob, and that digest was not found among the tracked versions of the
approval file in its Git history. The immutable approval decision therefore
cannot be verified from the current pointer alone.

The task source, pinned Mili commit, approved task IDs, evaluator source files,
and condition overlays were separately checked by content hash and match the
current freeze. This does not repair the approval/rubric chain.

## Run decision

Do not start additional Mili comparison runs from this freeze. Preserve the
existing four valid ledger entries as historical observations; this audit does
not retroactively rescore them. Before resuming, create a versioned immutable
approval snapshot and a reviewed rubric/freeze pair whose hashes can be
reproduced from committed bytes. Reconfirm any changed acceptance wording with
the user. Keep progress/results in a separate record so run updates do not
mutate the bytes identified as the approved snapshot.

The original Mili checkout and all existing pilot clones were left untouched.
