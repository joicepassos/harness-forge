# Security and publication readiness review

Date: 2026-09-18. Reviewed commit: `47926d9e741675b082c5560025d1693b1b81b2ac`, including the working-tree documentation changes present at the start of the review.

## Decision

Do not present this snapshot as a production-ready enterprise release. It is a useful developer CLI for controlled pilots on trusted repositories. Publication should wait for credential-output protection, filesystem containment, private-content exclusion, a supported patched build toolchain, installation verification, and a defined license and security policy.

The implementation is substantial, and the existing tests pass. Closed issues do not establish complete acceptance: several features have defects, reduced scope, or missing validation evidence.

## Scope and evidence

- Reviewed CLI commands, application boundaries, filesystem and HTTP adapters, plugins, schemas, tests, documentation, installers, release configuration, and local Git history. Examined the bodies of all 19 GitHub issues and relevant completion comments.
- GitHub currently reports the repository as private, with 17 closed issues (#1–#17) and two open issues (#18–#19). The only release returned was `v0.0.1`, published 2026-09-12; no 1.x release was available.
- `go test ./...` and `go vet ./...` passed on Windows amd64 with Go 1.26.2. The CLI built successfully; the authorized word-count plugin returned two words.
- Gitleaks 8.24.3 scanned all 31 commits reachable through local refs (`git --log-opts=--all --redact`): no leaks detected, approximately 405 KB scanned.
- Gitleaks also scanned a snapshot of all current tracked and nonignored untracked files: no leaks detected, approximately 373 KB scanned.
- Govulncheck 1.8.0 reported 11 vulnerabilities with reachable symbols in the Go 1.26.2 standard library. This is a toolchain finding, not 11 independently demonstrated attacks against this CLI.
- Local reproductions used synthetic documents and a fictional credential. No paid AI request or real-secret exfiltration was performed. Audit fixtures and an overlay test are under `.cache/security-audit`; product source was not changed.

Limitations: secret scanning does not prove the absence of every secret. It did not audit remote logs, deleted remote refs, unreferenced Git objects, private conversations, external systems, or every ignored/cache/binary artifact. Actual published binaries, native macOS/Linux installation, hosting-account security settings, and provider billing/retention contracts were not certified. The GitHub API returned no usable security-and-analysis settings; this must not be interpreted as proof that those features are enabled or disabled.

## Findings to resolve before publication

### F1 — High: provider errors can disclose an API key

`internal/llm/infrastructure/chatcompat/openai.go:105` includes the provider's `error.message` verbatim in the returned error. `stream.go:42` similarly exposes stream error text. The CLI prints errors, making terminal capture and CI logs additional disclosure channels.

A local HTTP fixture returned an HTTP 401 message containing a fictional active credential. `Generate` returned the complete credential in its error. This demonstrates an exposure path, not a historical leak or proof that every provider echoes keys.

Fix: expose safe status/category messages; redact active credentials consistently from errors and output before printing; cover normal, streaming, malformed-response, and retry paths with regression tests. The GitHub adapter already rejects responses containing its active token and provides a useful precedent.

### F2 — High: private documents can enter an index and AI context

`internal/indexing/infrastructure/local.go:21` selects Markdown/README documents without honoring `.gitignore` or applying a secret-path/content policy. `contextpack/infrastructure/build.go:294` applies filename heuristics, which cannot detect credentials embedded in otherwise ordinary source or documentation.

Reproduction: `private/secrets.md` was indexed even though `.gitignore` excluded `private/`. A fictional credential in `README.md` was included by `contextpack.Build`. Indexing is local, but selected excerpts can subsequently be sent by `rag`; `ask --repository` and discovery send selected context directly. The local index also stores document text in plaintext.

Fix: define a shared exclusion policy, honor repository and explicit user exclusions, reject known secret paths, inspect/redact sensitive content, and provide a preview of the exact material being transmitted. Do not promise that filename filters are a complete data-loss-prevention control. Treat indexes and generated reports as potentially confidential.

### F3 — High: `.harness` links allow writes outside the selected repository

`internal/harness/init.go:18` and `internal/indexing/infrastructure/local.go:145` create/write under `.harness` without checking whether that ancestor redirects outside the repository. Some other writers protect their final target but do not consistently validate all ancestors.

Reproduction on Windows: a junction named `junction-repo/.harness` pointed at a separate synthetic directory. Both `harness.Init` and `JSONStore.Save` succeeded and created `harness.yaml` and `index.json` in the outside directory. `init` retained its no-overwrite behavior, but location containment failed; index saving can replace the corresponding outside index.

Fix: enforce repository containment on resolved ancestors for every writer, reject unsafe symlinks/junctions/reparse points, and address races between checking and opening. Add native Windows and Unix tests. The attacker must be able to prepare/change the local directory structure; ordinary remote Git clones do not automatically carry Windows junctions.

### F4 — High release gate: vulnerable and obsolete build toolchains

The local source scan found these reachable standard-library advisories with Go 1.26.2: GO-2026-6218, 6090, 5972, 5856, 5039, 5037, 5026, 4986, 4977, 4971, and 4918. The highest minimum fix version reported for that branch was Go 1.26.6. Reachability is conservative and is not proof of practical exploitability for every advisory.

`.github/workflows/release.yml:35` and `:55` select Go 1.23.x. That branch is outside the current Go support window. Changing only the developer's installation would leave the release builder unaddressed.

Fix: select a currently supported patched Go version for release/CI, rebuild all artifacts, and run govulncheck against source and release binaries. Keep the module's minimum compatibility version distinct from the release builder's security policy.

Sources: [official Go release policy](https://go.dev/doc/devel/release#policy), [GO-2026-6218](https://pkg.go.dev/vuln/GO-2026-6218), [govulncheck documentation](https://go.dev/doc/security/vuln/).

### F5 — Publication blocker: archive configuration conflicts with installer expectations

Both installers require the archive to contain exactly one entry: `harnessforge` or `harnessforge.exe` (`install.sh:112`, `install.ps1:49`). `.goreleaser.yaml:26` does not configure archive `files`. GoReleaser includes README and LICENSE files by default. Consequently, archives produced with these defaults are incompatible with the current strict installer checks.

This is established by configuration and the [official archive documentation](https://www.goreleaser.com/customization/package/archives/); a complete GoReleaser snapshot/install round trip was not run in this audit. Resolve the configuration mismatch and verify actual archive bytes in clean installation tests before publishing. Do not weaken traversal and link checks to accommodate extra files.

The documentation also refers to `v1.0.0`, which is not an available release. Existing downloads require access to the private repository. These are additional first-user adoption barriers.

### F6 — High trust-boundary limitation: authorized plugins are unrestricted local programs

`internal/plugins/infrastructure/local.go:90` uses `exec.CommandContext`; its filtered environment does not establish an OS sandbox. A plugin runs with the user's filesystem/network permissions and can read accessible credential files, alter documents, or launch other processes. Capability declarations are metadata, not enforced OS permissions. Timeout cancellation does not establish process-tree containment.

The explicit `--authorize` requirement, discovery without execution, bounded output buffers, version checking, and credential-variable filtering are positive controls. The README already states the need for OS sandboxing. This is not an authorization bypass, but it does not meet a promise that scripts cannot affect users.

For enterprise use, disable unreviewed plugins and run approved extensions under a dedicated low-privilege identity or OS/container sandbox with explicit filesystem/network policy. Validate process-tree termination. Generated quality commands and agent instructions also require review: HarnessForge does not execute quality gates itself, but downstream agents may act on those instructions.

### F7 — Publication blocker: missing license and security policy

`LICENSE` is empty. There is no `SECURITY.md` in the repository. README BYOK guidance is useful but insufficient as a security policy. The claim that credentials are never stored is too broad when provider errors, shell history, redirected output, document content, and plugin access are considered.

Choose the intended license explicitly. Add a security policy that accurately describes current guarantees, supported versions, reporting route, update handling, and residual risks. Do not claim encryption, sandboxing, or complete secret prevention that the code does not implement.

### F8 — Medium: defensive limits are inconsistent

The index loader reads the entire JSON file; retrieval freshness reads paths supplied by index metadata without a containment/regular-file check; the Harness YAML loader has no explicit byte cap; historical evidence uses unbounded `git show` output; and SSE limits each line but not the aggregate buffered event. The analyzer also reads matching files without a byte cap before context selection applies its own limits.

These are static findings, not stress-tested denial-of-service demonstrations. Apply consistent size, type, path, time, and cancellation limits to all untrusted inputs. Treat `.harness/index.json` as untrusted input and validate source paths, chunk text/hash relationships, dimensions, and format versions before relying on its citations.

### F9 — Medium: release verification and maintenance controls are incomplete

Only a tag-triggered release workflow is checked in. All test jobs run on Ubuntu; GOOS/GOARCH are applied only to the later build step. The matrix cross-compiles five targets but does not execute tests on five native targets. There are no checked-in pull-request security, secret-scanning, or dependency-update workflows. Actions use movable major tags, and GoReleaser uses a floating major-version range. Signing/attestations are documented as future work.

Add PR checks, supported-toolchain vulnerability checks, redacted secret scans, reviewed dependency updates, native supported-platform tests, and clean installer smoke tests. Pin release dependencies and establish provenance verification. Verify branch/tag/environment protections separately in GitHub; repository YAML alone does not prove those account settings.

## Closed issue acceptance audit

"Implemented" below means code and behavioral tests support the core feature, not that the entire product is production-certified. Completion comments are corroborating records, not substitutes for tests or benchmarks.

| Issue | Assessment | Evidence and remaining gap |
| --- | --- | --- |
| [#1 Discovery](https://github.com/joicepassos/harness-forge/issues/1) | Partial; reproduced defect | Approval, evidence validation and duplicate-ID handling exist. `appendRule` appends at end of file instead of the actual `rules` sequence. A valid harness with `quality_gates` after `rules` rejects a new proposal with an unknown-field error. The existing file is preserved. Empty explicit `rules` and other layouts also need regression coverage. |
| [#2 Agent adapters](https://github.com/joicepassos/harness-forge/issues/2) | Core implemented; verification evidence incomplete | Deterministic rendering, approved-rule filtering and manual-file protection are tested. No dated acceptance record was found showing verification against the official agent formats required by the issue. |
| [#3 Structural symbols](https://github.com/joicepassos/harness-forge/issues/3) | Partial | Go uses `go/ast`. Java is a declaration regex/lexer, not a full parser: `class Demo { public void run() {} }` returns only the class; `class Broken {` returns success. Java method extraction and reliable syntax-error reporting do not satisfy the stated scope. |
| [#4 Embeddings](https://github.com/joicepassos/harness-forge/issues/4) | Core implemented; validation incomplete | API adapter, vector metadata, cosine similarity and invalid-vector handling exist. Tests use synthetic vectors/responses; no reproducible related-versus-unrelated sentence comparison was found. Live testing is explicitly optional. |
| [#5 Indexing](https://github.com/joicepassos/harness-forge/issues/5) | Core implemented; exclusion/security gap | Incremental reuse, deletion handling, source metadata and size limits exist. The index uses local lexical hashes, not the OpenAI semantic embedding adapter. Named directory exclusions exist, but gitignored/private Markdown is included (F2), and persistence has F3. No measured fixed-versus-structural chunking comparison was found. |
| [#6 Retrieval](https://github.com/joicepassos/harness-forge/issues/6) | Core implemented with limits | Ranking, path filters, K, stale-index checks and precision/recall calculations exist and are tested. This is lexical hash retrieval. Tests demonstrate mechanics; there is no representative enterprise retrieval-quality benchmark. Untrusted index handling needs F8. |
| [#7 RAG](https://github.com/joicepassos/harness-forge/issues/7) | Partial acceptance | Retrieval, context bounds, citation-ID validation, insufficient-evidence behavior, tokens, latency and direct mode exist. Current tests use fake retrieval/generation. Direct mode is not itself the required quality comparison against direct questions; no reproducible comparative quality results were found. It is exposed as `rag`, while `ask` retains the earlier context workflow. |
| [#8 Context engineering](https://github.com/joicepassos/harness-forge/issues/8) | Implemented with documented proxy | Selection, deduplication, serialized-payload budget, exclusions and provenance are tested. Cost is estimated; "quality" is prompt-relevance recall, not measured answer quality. Secret protection is filename based (F2). |
| [#9 Skills](https://github.com/joicepassos/harness-forge/issues/9) | Narrow implementation; gaps | A Java/backend heuristic proposes one procedure; approval, idempotence and manual preservation are tested. Generated content contains a description, evidence and limitations, with no detailed procedure. Any existing `skills:` section blocks adding a new distinct skill, including previously generated sections. Evidence labels such as "controller" are categories rather than extracted declaration names. |
| [#10 Evals](https://github.com/joicepassos/harness-forge/issues/10) | Implemented with limited rubric | Versioned fixtures, per-case/aggregate results, missing-case errors and baseline comparison exist. It scores supplied result files; correctness is term matching and faithfulness is citation membership, not semantic verification or an automated experiment runner. |
| [#11 Doctor](https://github.com/joicepassos/harness-forge/issues/11) | Core implemented | Read-only diagnostics, proposal-only fixes, duplicate/missing/oversized cases and limits are tested. Test discovery is Go-specific; historical-evidence reads retain F8. |
| [#12 GitHub knowledge](https://github.com/joicepassos/harness-forge/issues/12) | Core implemented | Read-only bounded API calls, pagination errors, comments/revisions, rate handling, token-echo protection and human-review candidates exist. Tests and completion comment support a prior authorized read-only smoke test. Decisions use explicit `Decision:` markers and repeated statements, not semantic consensus analysis. |
| [#13 Drift](https://github.com/joicepassos/harness-forge/issues/13) | Core implemented with disclosed limit | All evidence/revisions, aligned/different/not-evaluated states, explanations and proposals are tested. Matching is literal text, so it cannot prove architectural compliance. |
| [#14 Plugins](https://github.com/joicepassos/harness-forge/issues/14) | Contract implemented; no security sandbox | Authorization, discovery, compatibility, capabilities and failure handling exist. The example ran successfully in this audit. OS isolation remains an operator responsibility (F6). |
| [#15 English defaults](https://github.com/joicepassos/harness-forge/issues/15) | Implemented | Default CLI text, documentation and contributor convention are English; locale fixtures are separate. |
| [#16 Brazilian Portuguese](https://github.com/joicepassos/harness-forge/issues/16) | Implemented with incomplete coverage | Explicit selection, fallback and common help/error/output tests pass. Newly added command text can remain English: `--language pt-BR index --help` printed "Build an incremental document index". Do not claim full translation coverage. |
| [#17 Spanish](https://github.com/joicepassos/harness-forge/issues/17) | Implemented with incomplete coverage | Shared locale boundary and common-message tests pass. Catalog coverage is partial and English fallback remains part of the contract. |

[#18 TUI](https://github.com/joicepassos/harness-forge/issues/18) and [#19 distribution](https://github.com/joicepassos/harness-forge/issues/19) are correctly still open. There is no TUI. Issue #19 requests Homebrew/Scoop/npm distribution, whereas `docs/RELEASES.md` explicitly excludes these from the initial plan. Resolve that scope difference before claiming acceptance.

## Usability and enterprise fit

Strengths: a small Go dependency set; separable domain/application/adapters; offline deterministic commands; explicit review flags; structured output; inspectable evidence; local preferences that reject unsupported credential fields; and useful tests for several safety boundaries.

Current suitable use: a technical team running a controlled pilot, with trusted repositories, approved provider data flows, restricted credentials, plugins disabled unless independently isolated, and results reviewed by humans. It is a single-user CLI, not a multi-tenant enterprise service. JSON indexing has no concurrency/transaction guarantees for shared writers and is explicitly intended for small repositories.

Onboarding also needs correction: README's absolute `go run .../cmd/harnessforge` examples fail from an unrelated Go module (reproduced: "outside main module or its selected dependencies"). Build/install the executable first, then run it from the target repository. The available release and documentation must agree. Preserve the user's current documentation work while correcting these instructions.

## Required SECURITY.md content

The policy should describe:

1. Supported release versions and how security fixes are distributed, without inventing a support SLA.
2. A verified private reporting channel and responsible disclosure instructions; never direct reporters to paste secrets into public issues.
3. BYOK through process-scoped environment variables or an approved secret manager; avoid plaintext literal keys in shell history, command arguments, `.env` committed to Git, harness YAML, examples, logs or generated files.
4. Short-lived/scoped keys where available, separate development and production credentials, provider-side usage controls, and revocation/rotation after suspected exposure.
5. Clear disclosure that environment variables are not encrypted secret storage or protection from same-user/admin processes, debuggers and dumps.
6. Exact outbound data flows: AI prompts/context and embeddings go to the chosen provider; GitHub reads use its API; local analysis/index/search do not need text generation. Provider retention and organizational approval must be assessed separately.
7. Local data artifacts and handling: indexes, prompts, output, generated instructions, logs and caches may contain repository information.
8. Plugin trust, filesystem/link limitations, prompt-injection risks, human review of generated instructions, and verified installation/update procedures.
9. Known limitations and an incident response procedure. Documentation must describe actual behavior, and cannot substitute for fixes F1–F4.

## Recommended implementation order

1. Fix F1–F3 and add focused regression tests; unify credential-output and repository-boundary protections.
2. Upgrade/rebuild the release toolchain; add vulnerability/secret checks and native installer tests; reconcile archive contents and installer validation.
3. Define license and reporting/support policy, publish accurate SECURITY.md, and reconcile release/onboarding documentation.
4. Repair discovery YAML insertion and Java extraction/error handling; make issue acceptance and remaining scope explicit.
5. Add representative end-to-end retrieval/RAG evaluation and verify a limited enterprise pilot before broad 1.x publication.

No application fixes, issue state changes, commits, pushes, or release publication were performed by this review.
