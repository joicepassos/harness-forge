# Interactive CLI verification

Date: 2026-10-02. Branch: `codex/mvp-cli-feedback`, based on PR #40
commit `0dac6890254277b88a4a7190c5f2f0d93ec7cd20`.

## Implementation

- Huh v2.0.3 handles language, language adjustments, provider/model/agent
  selectors, confirmations, hidden credentials, and notes.
- Bubble Tea v2.0.10 and Bubbles v2.2.1 provide the context browser with
  directory navigation, persistent multiple selection, review, and removal.
- Interactive mode requires terminal input and output. `--accessible` on
  `init`, `install`, or `tui`, or `HARNESSFORGE_ACCESSIBLE=1`, uses plain prompts.
  Redirected input/output also uses plain prompts.
- Context limits, sensitive-file filters, root confinement and API consent
  are retained. Advanced path entry remains available in the browser.

## Results

- `go test ./...`, `go vet ./...` and diff whitespace checks passed.
- Windows terminal: Portuguese language, detected Go, local proposal,
  directory navigation, marking a file, reviewing/removing it, and confirming
  generation of Codex and Claude instructions completed on `demo-usuario-mvp`.
- Windows terminal: DeepSeek provider/model selection, environment-key
  acknowledgement, advanced paths, declining context transmission, and
  cancelling at the final preview completed without writing files.
- Demo validation and `check --layout harness --run-gates` passed.
- Browser tests cover directory selection, navigation, review, cancellation,
  sensitive content, root navigation and narrow viewport dimensions.
- Existing scripted setup and local-fallback tests passed.

## Remaining limits

- The escape-symlink test skipped on Windows because creating links requires
  a privilege unavailable to this process. Root navigation tests ran separately.
- Browser selections still use the MVP's supported text-document extensions;
  source-code files and binaries are not shown as selectable documents.
- New terminal checks ran on Windows, not macOS/Linux. Hidden-key entry was
  not exercised live because the provider key was already present.
- No external API request was made in this UI verification round. Historical
  DeepSeek HTTP 400 diagnosis remains unchanged from the preceding correction.

## Follow-up: feedback tasks 7 and 8

The updated CLI was built and exercised on `demo-usuario-mvp` with DeepSeek
`deepseek-flash`. After context consent, the interactive terminal showed
context prepared, waiting for the provider with an animated indicator and
elapsed seconds, response received, and proposal validation. The provider
responded after approximately 40 seconds. No credentials or document content
were included in these progress messages, and no percentages were displayed.

One skill had invalid evidence and was discarded. Eight rules and two skills
with verified citations survived and were presented for review. Generation
of Codex and Claude instructions completed. Demo validation and the check
with Go test gates passed. The demo's application code was not changed.

Recovery validates the entire structure before filtering citations and
validates the remaining proposal again. It does not rewrite sources or quotes,
and it makes no additional paid request. If no cited item survives, the CLI
explains that the AI failed to cite a project file correctly and offers local
generation. Empty rule/skill arrays originally returned by the provider are
still valid. Citation verification does not replace human semantic review.

`go test ./...`, `go vet ./...`, and whitespace checks passed. Focused tests
cover noninteractive progress without terminal sequences, elapsed time,
cancellation and worker shutdown, partial recovery, all-invalid local fallback,
nonexistent sources, fabricated quotes, duplicate IDs, and structural errors.
The complete live rejection path was covered by deterministic tests; the live
provider response exercised partial recovery instead. This follow-up's terminal
verification was on Windows only.

## Follow-up: background call inspector

The AI stage of interactive setup now runs in an independent process and
opens the call inspector after context consent. Explicit `--background`
returns immediately. The inspector has context, calls, validation and
proposal sections, scrolling, elapsed time and real timestamped stages.
HTTP events include actual attempts and status; response headers and completed
response bodies are distinguished. Detaching does not cancel the worker.
Applying requires a separate reviewed `init resume` confirmation.

Windows live checks on the demo project:

- Detached DeepSeek `deepseek-flash` generation returned the terminal before
  completion. A separate inspector reopened the run successfully.
- HTTP 200 headers arrived after 593 ms; the complete proposal arrived after
  approximately 26 seconds. Seven rules and four skills survived strict
  citation validation; one invalid rule was discarded.
- Review refusal created no `.harness`. A later approved resume generated
  the harness, four skills, Codex and Claude instructions. Validation and
  `check --layout harness --run-gates --format json` passed.
- Interactive Portuguese setup opened the inspector automatically with
  Ollama. The unavailable local server produced a real transport failure;
  detaching succeeded and resuming offered the local proposal. That local
  review was declined, leaving the project unchanged at that point.
- The saved DeepSeek run contained no provider credential. Document contents
  did not appear in inspector output; accepted citation excerpts appear only
  in explicit proposal review and the privately cached proposal.
- Demo tests and execution passed: Estacao Aurora, average 22.0 C. No demo
  application source was changed.

Automated coverage includes per-attempt HTTP events and retries, privacy,
malformed worker input, authorized-context integrity, active cancellation,
stale-worker recovery, review refusal, local fallback, changed documents,
fabricated citations, successful application and refusing reapplication.
Inspector tests bound both width and height at 20/40/80 columns and 10/20
rows, check timestamp ordering and distinguish headers from full responses.

Limits: live verification was Windows-only; Linux/macOS were not exercised.
Cancellation and HTTP retries were covered deterministically, not with extra
paid calls. Crash recovery uses a six-minute stale threshold. Raw observations
are not retained for resumed generation, and cache storage is not encrypted
by the CLI. Context checks cover the analyzer's findings and the authorized
document text (including the existing 16 KiB per-document cap), not an
immutable snapshot of every project file. Citation validity still requires
human review of the proposal's meaning.
