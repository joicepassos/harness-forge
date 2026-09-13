# HarnessForge

HarnessForge is a Go CLI for deterministic repository analysis, AI-assisted architecture review and editable agent harness files.

## Install And First Commands

Download the release binary for your system or run from source with Go 1.23+:

```powershell
go run ./cmd/harnessforge version
go run ./cmd/harnessforge init
go run ./cmd/harnessforge analyze --git --format json C:\path\to\your-project
```

`init` creates `.harness/harness.yaml` and refuses to overwrite an existing file. `analyze` performs deterministic local analysis. With `--git`, it includes known local and remote branches, pending files, authors, commit counts and the messages/files from the latest 20 commits. Repositories without commits are accepted. Git errors are reported rather than discarded, and no fetch is performed.

## CLI Language

Spanish is available with `--language es`, for example `harnessforge --language es validate`. All languages share the same commands and output formats; English remains the default and the fallback for missing translations.

English (`en`) is the default. Use the global `--language pt-BR` option for Brazilian Portuguese help and common command messages. Unsupported language values fail clearly; no language preference is written to project files or the Harness IR.

Language selection is explicit and independent of the operating system locale. Missing translations fall back to English. Help headings, command descriptions, option descriptions, and common validation messages are localized; machine-readable JSON fields, evidence, provider output, and underlying operating-system diagnostics retain their original wording. Invalid language options fail even with `--help` before any command executes.

```powershell
go run ./cmd/harnessforge --language pt-BR --help
go run ./cmd/harnessforge --language pt-BR analyze --format json C:\path\to\your-project
```

## AI And BYOK

```powershell
go run ./cmd/harnessforge config set provider deepseek
go run ./cmd/harnessforge config get provider
go run ./cmd/harnessforge ask "Hello"
go run ./cmd/harnessforge ask --stream "Explain Strategy in one sentence"
go run ./cmd/harnessforge ask --format json --repository C:\path\to\your-project "List patterns supported by the selected context"
go run ./cmd/harnessforge context explain C:\path\to\your-project "How are webhooks authenticated?"
```

| Provider | Environment variable | Default model |
| --- | --- | --- |
| openai | OPENAI_API_KEY | gpt-4o-mini |
| deepseek | DEEPSEEK_API_KEY | deepseek-v4-flash |
| gemini | GEMINI_API_KEY | Provide `--model` |
| groq | GROQ_API_KEY | Provide `--model` |
| ollama | None for a local server | Provide `--model` |

API keys stay only in the process environment. `.env` files are not loaded. Ollama must be running at `localhost:11434` with the selected model installed. The global provider preference is saved as `harnessforge/preferences.json` under `os.UserConfigDir` (Windows: `%APPDATA%`). Only the provider name is persisted. `--provider` overrides the saved preference for one run; without a preference, HarnessForge uses OpenAI.

Text mode sends only the prompt. `--repository`, available with `--format json`, builds a bounded repository context and sends only selected excerpts plus the prompt to the chosen provider. No query modifies the repository.

### Context Selection And Explain

Repository context is selected before a structured model call. HarnessForge ranks deterministic analyzer findings and bounded file excerpts by prompt relevance, breaks ties deterministically, deduplicates identical excerpt text while retaining all origins, compresses oversized excerpts, and enforces a budget against the serialized source payload sent to the provider. The estimator is `payload-byte-upper-bound-v1`: one UTF-8 byte is counted as one budget unit after JSON serialization, plus a 140-unit payload framing reserve. This is intentionally an upper bound for source payload bytes, not a tokenizer-specific model token count. The budget excludes the system prompt, embedded JSON schema and expected model output; actual API usage is still reported by the provider separately where available.

Use `context explain` to inspect the retrieval boundary without contacting an AI provider:

```powershell
go run ./cmd/harnessforge context explain C:\path\to\your-project "How are webhooks authenticated?" --budget 1800
go run ./cmd/harnessforge ask --format json --repository C:\path\to\your-project --context-explain "How are webhooks authenticated?"
```

The explain output includes included and excluded excerpts with source IDs, paths when safe, relevance scores, estimated budget units, rank, exclusion reasons, duplicate links, compression provenance and retained origins. Secret-like files, symlinks, binary or non-UTF-8 files, unreadable files and files beyond the scan limit are represented as excluded metadata without file contents. The comparison block reports the actual previous analyzer JSON source payload cost, the broader unfiltered candidate corpus cost, selected cost, estimated reductions and deterministic relevant-source recall for the previous analyzer-only source set and the selected source set. Recall is a proxy over prompt-matched candidates, not a measurement of answer quality.

The current retrieval layer is local and heuristic. It does not embed code, execute semantic search or prove that the highest-ranked excerpt is architecturally correct. Human review and evidence validation remain required.

### Skill Discovery

Discover recurring backend development procedures without changing the repository:

```powershell
go run ./cmd/harnessforge skill discover C:\path\to\your-project
```

The command returns candidate procedures with file and symbol examples plus limitations. It currently recognizes a Java backend chain containing controller, service, persistence, database migration and integration-test evidence. It is a heuristic, not proof that the procedure is required or correct.

After human review, pass the selected proposal JSON to `skill generate` with `--approve`. Generation creates `.harness/skills/<id>/SKILL.md` and adds an approved, evidence-backed reference to the Harness IR. Existing generated IDs are left unchanged. A pre-existing manual `skills` section or skill directory is never overwritten and must be updated manually.

### Evaluation

Run a deterministic evaluation with a versioned YAML dataset and a versioned results file:

```powershell
go run ./cmd/harnessforge eval run examples/evals/dataset.yaml examples/evals/results.yaml
go run ./cmd/harnessforge eval compare baseline-report.json candidate-report.json
```

Each report records dataset, index, model, prompt and rubric versions. It reports per-case and aggregate Recall@K, Precision@K, deterministic correctness, citation-grounding faithfulness, tokens, latency and visible failures. Retrieval uses exact source IDs; correctness matches required terms; faithfulness only verifies cited retrieved sources against the expected source set. These measures cannot establish answer truth, and no LLM judge is used. Human review remains necessary.

### Structured AI Output

`ask --format json` requests JSON from the provider and validates it locally with `schemas/architecture-analysis.schema.json`. Output contains `architecture` and `patterns`; each pattern requires a name, confidence from 0 to 1 and literal citations from the prompt or selected repository context. Extra fields, duplicate keys, truncated output, citations absent from selected context and invalid JSON are rejected before anything is written to stdout.

Architecture styles v1 are `ddd`, `hexagonal`, `layered`, `clean-architecture`, `event-driven`, `microservices`, `monolith` and `mvc`. Each architecture style needs a pattern with the same name and evidence. Technologies belong in `patterns`. If evidence is insufficient, the model should return empty arrays.

Confidence is an uncalibrated model estimate, not observed frequency. A literal citation proves only that the quoted text was in the selected context; it does not prove the architectural interpretation is correct.

The adapter uses JSON mode and local validation without relying on native JSON Schema support from every provider. Compatibility depends on the selected model. `--stream` applies only to text; JSON output is withheld until it passes validation.

### Failures And Cancellation

HTTP 429, 500, 502, 503 and 504 responses are retried up to three times with progressive waits and `Retry-After` support. If the requested wait is longer than 10 seconds, the error is returned. Each call has a 60-second timeout. Authentication and balance errors, ambiguous connection failures, invalid content and streams that already started are not retried. Ctrl+C cancels the operation. Streaming requires a terminator and normal completion; failures after displayed chunks are reported.

## Harness IR: Manual Editing And Review

```powershell
go run ./cmd/harnessforge validate
go run ./cmd/harnessforge validate examples/sample.harness.yaml
go run ./cmd/harnessforge validate path\harness.yaml --repository C:\path\to\your-project
go run ./cmd/harnessforge review my-rule approved --file path\harness.yaml
go run ./cmd/harnessforge review my-rule candidate --file path\harness.yaml
```

`validate` reads `.harness/harness.yaml` by default. It validates the embedded `schemas/harness-v1.schema.json` schema and domain invariants. Only `version: 1` and `project.name` are required in a minimal document. Optional sections are `project.languages`, `architecture.styles`, `rules`, `skills` with `id` and `description`, and `quality_gates` with `id` and `command`.

Rules require `id`, `description`, `origin` (`human` or `ai`) and `status` (`candidate`, `approved` or `rejected`). IDs are unique per collection. AI rules require evidence with `file`; `symbol` and `revision` are optional. Human rules do not require confidence or evidence of existing patterns. `scope.paths` is metadata and does not execute filters in this phase.

`validate --repository` checks that referenced files exist and that symbols appear literally. With `revision`, it checks the content at that Git revision. Paths must be relative to the repository root; directory escapes are rejected. This is not an AST analysis and does not prove a rule is true. Quality gates are declarations only and are never executed by `validate` or `review`.

`review` supports candidate to approved/rejected and approved/rejected back to candidate. A decision must be reopened before it is inverted. The edit changes only the status scalar, preserving comments, order, quotes and line breaks; it uses a temporary file, an operation lock and concurrent change detection. Status values with aliases, anchors or multiline syntax must be edited manually. There is no general YAML reserialization in this version.

Manual editing can declare any valid status. There is no approver authentication. Git history provides traceability. The included example is illustrative and contains a candidate rule, not an adopted project decision.

### Harness Health Diagnostics

Diagnostics report invalid repository roots and incomplete scans explicitly. Repository scanning checks at most 20,000 entries and skips `.git`, `vendor`, and `node_modules`; test detection currently recognizes only regular Go test files. The diagnostic input limit is 4 MiB. Every rule and skill evidence entry is checked independently, and skill references resolving outside the repository are rejected.

```powershell
go run ./cmd/harnessforge doctor .harness/harness.yaml --repository C:\path\to\your-project
go run ./cmd/harnessforge doctor --fix
```

`doctor` is read-only. It emits JSON diagnostics with severity, evidence and a suggested action for invalid Harness IR, potentially duplicate instructions, unsafe declared paths, invalid local evidence and oversized Harness IR. `--fix` does not edit files: it labels the output as proposals for a human to review and apply manually. The context warning threshold is 65,536 Harness IR bytes; it is a byte-size boundary, not a token count or health score. Diagnostics do not establish that instructions are correct, and text similarity can identify intentional repetition. Evidence verification requires `--repository` and checks only local paths, revisions and literal symbols.

### GitHub Knowledge Extraction

```powershell
$env:GITHUB_TOKEN = "your-token"
go run ./cmd/harnessforge github learn owner/repository
```

`github learn` only sends bounded GET requests to GitHub and never posts, edits or approves content. The repository is supplied as `owner/repository`; the token is read from `GITHUB_TOKEN` (or `--token-env`) only at runtime and is never written to the Harness IR, configuration, or output. The report preserves source URLs, pull revisions and comment context, and marks every extracted candidate as requiring human review. Only text explicitly prefixed with `Decision:` is classified as a decision; recurring discussion or model interpretation is not proof of a rule. Requests are cancellable, time out after 20 seconds, fetch at most 10 pages per endpoint, and reject individual API responses above 16 KiB. These safety limits can omit content; rate-limit and API errors are returned explicitly.

### Drift Detection

Each evidence entry is evaluated separately. Declared Git revisions are resolved and checked as historical baselines before comparing the working copy; invalid baselines and reader failures are marked `not_evaluated`. Reports include the declared revision and baseline status. Historical and current content reads are bounded to 4 MiB, and cancellation propagates to Git. A missing current file or symbol is a difference requiring review, with both legitimate-change and violation explanations.

```powershell
go run ./cmd/harnessforge drift .harness/harness.yaml --repository C:\path\to\your-project
```

`drift` is read-only. Its first structural strategy evaluates approved rules with literal evidence symbols, reporting an aligned location when the symbol remains present. Rules without a literal symbol are explicitly not evaluated. A missing symbol is reported as a difference, never automatically as a defect: the output presents violation, intentional architectural change, and stale-rule explanations together with separate review-only code-fix and Harness-update proposals. It does not edit code, evidence, statuses, or manual Harness IR content. It rejects unsafe evidence paths and symlinks outside the repository. The initial check is literal text matching, not AST analysis or proof of architectural compliance.

## Architecture And Validation

Domain packages define contracts, messages, Harness IR and context data. Application packages coordinate use cases and selection strategies. Infrastructure packages implement HTTP, filesystem, Git, YAML and persistence. Provider selection uses Strategy; providers with the same protocol share the Chat Completions adapter.

```powershell
go test ./...
go vet ./...
```

Tests cover schemas, citations, HTTP failures, retry, truncated streams, Git with and without commits, review preservation, context budgets, duplicates, oversized context, irrelevant documents and security boundaries for path traversal, secret-like files, symlinks, binary files and cancellation. DeepSeek was validated live for JSON and streaming in an earlier phase. OpenAI, Gemini, Groq and Ollama have simulated tests; real execution requires each environment's credentials, balance or local server. Anthropic remains an optional integration that is not implemented.

References: [Components of a Coding Agent](https://magazine.sebastianraschka.com/p/components-of-a-coding-agent), [Go context patterns](https://go.dev/blog/context), [Go fuzzing and security testing](https://go.dev/doc/security/fuzz/), [OpenAI structured outputs](https://developers.openai.com/api/docs/guides/structured-outputs), [DeepSeek chat completion](https://api-docs.deepseek.com/api/create-chat-completion/), [Gemini OpenAI compatibility](https://ai.google.dev/gemini-api/docs/openai), [Groq OpenAI compatibility](https://console.groq.com/docs/openai), [Ollama OpenAI compatibility](https://docs.ollama.com/api/openai-compatibility).
