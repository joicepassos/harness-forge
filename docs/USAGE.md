# Using HarnessForge

This guide is the practical documentation for HarnessForge. All commands are shown with `go run ./cmd/harnessforge`; replace that prefix with `harnessforge` if you installed or built the executable.

Run project-facing commands from the target repository unless a repository path is provided explicitly.

Start in the target repository with `harnessforge init`. In v1.1.0 and newer, the command runs a guided setup: it analyzes the repository, accepts additional text documents and observations, asks for an AI provider only if you choose an AI proposal, previews all generated files, and writes only after confirmation. `harnessforge install [repository]` and `harnessforge tui [repository]` are compatibility aliases. The npm package also exposes `harness-forge init`.

## Before you begin

You need Go 1.23 or later to run from source. Start by checking that the CLI is available:

```powershell
go run ./cmd/harnessforge version
go run ./cmd/harnessforge --help
```

For a repository outside the HarnessForge clone, build or install the executable and call it from that repository.

## 1. Understand a repository

Use `analyze` for a deterministic, local snapshot. It does not call an AI provider or modify the repository.

```powershell
go run ./cmd/harnessforge analyze C:\work\my-project
go run ./cmd/harnessforge analyze --git --format json C:\work\my-project
```

Choose `text` (the default) for a quick read, or `json` when you want to save, filter, or pass the result to another tool. `--git` includes local and remote branch names, pending files, authors, commit counts, and information from the latest 20 commits. It never fetches.

## 2. Configure the project

Run the guided setup once in the target repository:

```powershell
Set-Location C:\work\my-project
harnessforge init
```

The CLI shows detected languages, build tools, frameworks, test signals, and top-level directories. Add UTF-8 text files or directories when prompted; PDF and Word documents must first be exported as text. The CLI skips secret-like paths and content, bounds the context size, and shows which documents will be sent to the selected AI provider. You can add free-form team observations, choose Codex, Claude, or both, and review the exact files before confirming. If you do not use AI, it still creates a local project configuration and agent instructions.

The resulting `.harness/harness.yaml` is the editable source of truth for agent instructions. A small manual rule looks like this:

```yaml
version: 1
project:
  name: my-project
rules:
  - id: run-tests-before-review
    description: Run the project test suite before requesting review.
    origin: human
    status: approved
```

Validate before generating instructions:

```powershell
harnessforge validate --repository .
harnessforge generate codex --repository .
harnessforge generate claude --repository .
```

The generated files include approved rules only. Keep editing the YAML for long-term decisions; regenerate the agent files after approved changes. Generation refuses to replace files it does not own, so manual instruction files remain protected.

## 3. Search project documentation and ask grounded questions

Create an index of Markdown and README files, then search it locally:

```powershell
harnessforge index .
harnessforge search . "How is authentication configured?" --k 5
```

For an answer backed by retrieved project excerpts, configure a provider key in the current shell and use `rag`:

```powershell
$env:OPENAI_API_KEY = "your-api-key"
harnessforge rag . "How is authentication configured?" --k 5
```

`rag` validates that provider citations refer to retrieved chunks. If retrieval has no overlap, it returns `insufficient_evidence` instead of asking the provider. The index is local at `.harness/index.json`; do not commit a key to it or to any project file.

## 4. Propose and approve a rule

Use discovery when you want an AI-assisted suggestion with repository evidence, while keeping the decision with a human reviewer:

```powershell
harnessforge discover propose . "What conventions should contributors follow for database migrations?"
```

Review the returned proposal JSON. To apply a proposal, pass it to `discover apply` with `--approve`, a target repository, and the harness file. Check the exact arguments with:

```powershell
harnessforge discover apply --help
```

The proposal command is read-only. Applying requires explicit approval and validates each cited file and symbol before making an atomic change. You can later change a rule's review state without reformatting the YAML:

```powershell
harnessforge review <rule-id> approved --file .harness/harness.yaml
```

## 5. Validate, diagnose, and check for drift

Use these commands as the harness grows:

```powershell
# Confirm schema rules and referenced evidence.
harnessforge validate --repository .

# Report issues and suggested human actions; this is read-only, even with --fix.
harnessforge doctor .harness/harness.yaml --repository .

# Compare approved evidence with the working repository; this is read-only.
harnessforge drift .harness/harness.yaml --repository .
```

Drift findings are prompts for review, not automatic defects. A changed symbol might be a regression, an intentional redesign, or evidence that the harness needs an update.

## AI provider setup

Set a key only in the shell that runs the command, then select a provider if needed:

```powershell
$env:OPENAI_API_KEY = "your-api-key"
harnessforge config set provider openai
harnessforge ask "Summarize this design in one sentence."
```

Available providers and their environment variables are listed in the [Security policy](../SECURITY.md#credentials-and-byok). Use `ask --format json --repository .` for a locally validated architecture response with bounded source context, or `context explain` to inspect that context without contacting a provider.

## Command map

| Area | Commands | Typical use |
| --- | --- | --- |
| Repository inspection | `analyze`, `symbols` | Understand code and repository state |
| Harness lifecycle | `init`, `validate`, `review`, `generate`, `doctor`, `drift` | Maintain reviewed agent instructions |
| AI and context | `config`, `ask`, `context`, `embedding` | Configure a provider and inspect bounded context |
| Document retrieval | `index`, `search`, `rag` | Find and answer from project documentation |
| Assisted discovery | `discover`, `skill`, `github` | Produce reviewable rules or procedure candidates |
| Evaluation | `eval` | Compare retrieval and answer quality reports |
| Extensions | `plugin` | Discover or explicitly run a versioned plugin |

Every command supports `--help`. Brazilian Portuguese and Spanish help are available with `--language pt-BR` and `--language es`.

## Safe working habits

- Run `validate --repository .` before generating agent instructions.
- Treat AI proposals, generated text, and retrieved documents as review material—not proof.
- Use `context explain` when you need to see the exact repository evidence selected for an AI request.
- Keep API keys in environment variables only. HarnessForge does not load `.env` files.
- Commit `.harness/harness.yaml` when it represents shared project decisions; review generated instruction changes like any other code change.
