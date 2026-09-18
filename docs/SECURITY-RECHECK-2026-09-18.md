# Security correction comparison

Date: 2026-09-18. Baseline: `47926d9`. Rechecked: `3ade606` and existing working-tree documentation. Five implementation commits changed 41 tracked files relative to the baseline.

## Verdict

The changes resolve several original reproductions and materially improve documentation and CI configuration. They do not yet close the security/publication review. Keep the scope at a supervised pilot; do not claim all original findings are resolved.

## Original findings compared

| Original finding | Current result | Evidence |
| --- | --- | --- |
| F1: provider errors expose credentials | Original HTTP and SSE error-message paths fixed; protection still incomplete | Independent local HTTP tests confirmed that `error.message` is no longer echoed. A fictional active key supplied as `finish_reason` is still included verbatim in the returned error by `validFinish` in `internal/llm/infrastructure/chatcompat/openai.go`. This is a provider-controlled output path, not evidence of an actual historical leak. |
| F2: private files/context | Partial; still a release blocker | The original root `private/` exclusion now works. However, root `private/` fails to exclude `docs/private/notes.md`; nested `sub/.gitignore` is not read; and `password = "fictional-password-for-audit"` and JSON `{"api_key":"fictional-credential-for-audit"}` are not recognized by the new content filter. All corresponding synthetic documents were indexed. `credentials.md` is also accepted by the indexer when content does not match its regex. |
| F3: writes through `.harness` links | Original reproduction fixed | Repeated the Windows junction test: both init and index save now reject the redirected `.harness`. The helper checks existing components but still returns a path subsequently reopened by writers; this does not establish race-resistant containment. The new committed link tests skip Windows, so retain native junction regression coverage. |
| F4: vulnerable toolchain | Not resolved | Both workflows now explicitly select Go 1.26.2, the same version for which the original audit reported 11 reachable standard-library advisories. Updating CI from 1.23 to 1.26.2 does not resolve those findings. `docs/RELEASES.md` describes it as patched; this wording is inconsistent with the previous scan. A fresh vulnerability scan was not completed in this recheck. |
| F5: installer/archive mismatch | Changed; not validated | `.goreleaser.yaml:34` now says `files: none`. Official documentation defines `files` as a list and demonstrates `files: ["none*"]` for binary-only archives. The scalar form should not be accepted as a verified correction without a successful `goreleaser check` and an actual package/install round trip. No GoReleaser executable was available locally for this recheck. |
| F6: plugins unrestricted | Risk documented; behavior unchanged | README and SECURITY.md now correctly explain that authorization and subprocesses do not provide an OS sandbox. There is no new enforced sandbox or process-tree containment. External isolation remains necessary for the pilot. |
| F7: license/security policy absent | Substantially resolved | MIT LICENSE and SECURITY.md now exist. The policy describes BYOK, process environment limitations, outbound data, plaintext artifacts, plugins, reporting, supported versions and incident response. The reporting endpoint's availability to external reporters was not independently verified. |
| F8: inconsistent input limits | Partial | Index/YAML reads and buffered SSE events have limits; index metadata and retrieval containment received checks. However, `internal/harness/infrastructure/evidence.go:44` still uses unbounded `git show` output. The changes to the drift reader do not fix that separate evidence reader, which discovery and review use. ReadFile's byte cap alone also does not reject a non-regular blocking input. |
| F9: release maintenance controls | Improved configuration; gates incomplete | PR checks, native OS jobs, pinned action references, candidate packaging, installer smoke jobs and attestations are present in YAML. Their execution and reference validity were not confirmed remotely. The release job runs `goreleaser release --clean` before scanning release binaries, so that later scan cannot prevent publication of an artifact it rejects. Scan and attest the exact artifacts before publishing them. |

For the archive configuration, see [GoReleaser's documented binary-only packaging](https://www.goreleaser.com/customization/package/archives/#packaging-only-the-binaries). This comparison distinguishes a documented configuration discrepancy from a locally reproduced GoReleaser failure.

## Feature acceptance compared

| Earlier gap | Current result |
| --- | --- |
| Discovery fails when `quality_gates` follows `rules` | Fixed in the original reproduction. Existing and explicit-empty rule sequence tests pass. Other YAML styles are not fully covered by that result. |
| Java misses methods and accepts an unclosed class | Both original examples now behave correctly. However, `class Demo { void run() { client.send(); } }` reports both `run` and `send` as methods, even though `send` is only a call. The regex remains unsuitable as reliable declaration evidence. |
| Agent adapter format verification (#2) | Still missing the relevant record. The new ADAPTER-VALIDATION document verifies chat API fields; issue #2 concerns generated AGENTS.md and CLAUDE.md formats. These are different adapters. |
| Related/unrelated sentence embedding comparison (#4) | No new embedding comparison test or result was added. |
| RAG quality comparison (#7) | Documentation was added, but the baseline/candidate reports were not regenerated or backed by a new experiment. Both still have empty `cases` arrays and manually supplied aggregates; `results.yaml` contains supplied synthetic answers. These demonstrate report arithmetic, not direct-versus-retrieval answer quality. |
| Additional skills and useful procedure content (#9) | Unchanged: an existing `skills:` section prevents adding another distinct skill; rendering still consists of description, evidence and limitations. |
| Localization gaps (#16/#17) | A limitation document was added; catalogs/command translations were not completed by these commits. |
| Public distribution scope (#19) | Documentation now explicitly defers Homebrew/Scoop/npm from the controlled pilot rather than removing them from the public-release scope. Implementation is still pending. |
| Onboarding from another repository | Current working-tree README still contains absolute `go run .../cmd/harnessforge` examples. The previous outside-module failure remains applicable; use a built/installed executable. |

## Reproductions and checks

- `go test ./...`: passed on Windows amd64 with local Go 1.26.2.
- `go vet ./...`: passed.
- Repeated prior audit fixture runner: root ignore case no longer indexed; discovery insertion succeeded; basic Java method extracted; unclosed Java class rejected; `.harness` junction writes rejected.
- Independent HTTP/SSE tests: original error text safely suppressed; `finish_reason` still exposes the fictional key supplied by the test server.
- New isolated fixture runner confirmed nested-ignore failures, quoted/JSON credential-filter failures, and Java invocation misclassification.
- Reproduction files are in `.cache/security-audit/recheck.go`, `recheck_chat_test.go`, and `recheck-overlay.json`. These are audit-only files; no product source was edited.
- All credentials/content used for reproduction were fictional. No live AI request or real-secret transmission was performed.
- No fresh claim of "no leaked keys" is made for the new commits: the previous Gitleaks result covers the previous 31-commit snapshot, not automatically the new changes.

## Verification limits

Automatic approval review rejected the GitHub inspection command because of an account usage limit. Remote CI runs, issue state changes, action commit validity, release availability and repository protections remain unverified. The planned fresh network-enabled scanner call was not reached after that failure. These checks were not bypassed through another route. Official public GoReleaser documentation was consulted separately to assess the local archive configuration.

## Next corrections

1. Upgrade the actual release builder beyond the previously flagged Go patch level, then rerun source and binary vulnerability scans against the current database.
2. Correct ignore semantics and quoted/structured credential handling, applying one exclusion policy consistently to index and context. Add the failing synthetic cases as regressions.
3. Finish safe provider-output handling, including unexpected completion metadata.
4. Correct and validate GoReleaser configuration; scan the exact final binaries before publication, with successful native installer jobs.
5. Fix the separate historical-evidence reader and Java declaration detection; complete acceptance evidence rather than relabeling old synthetic reports.

This recheck did not modify product code, close issues, publish releases, or overwrite the earlier audit record.

## Follow-up correction

After this recheck, the remaining reproduced findings were corrected locally:

- Provider-controlled completion metadata no longer echoes an unexpected finish reason.
- The shared exclusion boundary now recognizes quoted and JSON credential values, credential-bearing paths, root directory rules at every depth, and nested `.gitignore` files.
- The Java matcher requires a declaration-like return type and no longer classifies `client.send()` as a method declaration.
- Historical Git evidence is collected through a 4 MiB bounded writer.
- CI and release workflows now use Go 1.27.1 and govulncheck 1.8.0. Release artifacts are built without publication, scanned, attested, then uploaded through `gh release create`.
- The GoReleaser binary-only archive configuration now uses the documented nonmatching `none*` file glob.

`go test ./...` and `go vet ./...` passed after these corrections on the local Go 1.26.2 development toolchain. The release workflow's Go 1.27.1 execution and actual clean installer jobs still require CI confirmation before release.
