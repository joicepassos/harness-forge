# Security policy

## Supported versions

Security fixes are released in the newest published `v1.*` release. Development builds, untagged commits, and releases outside the current `v1.*` line are not supported. There is no response-time or fix-time SLA.

When a fix is available, the project publishes a new versioned GitHub Release; it does not replace the bytes, checksums, or tag of an existing release. Operators should update to the fixed release, revoke or rotate any credential that may have been exposed, and follow the release notes for recovery steps.

## Reporting a vulnerability

Use the repository's private GitHub Security Advisory form: <https://github.com/joicepassos/harness-forge/security/advisories/new>. This route creates a private draft advisory visible to the repository maintainers, rather than a public issue. Maintainers verified authenticated access to the repository's advisory area on 2026-09-18; if the form is unavailable to you, do not fall back to a public issue.

Do **not** report a vulnerability in a public issue, discussion, pull request, commit, CI log, or chat transcript. Do not send live API keys, access tokens, passwords, private source material, or customer data. Instead, provide redacted steps to reproduce, affected version or commit, impact, and safe evidence such as a synthetic fixture. If a secret was exposed, revoke it before reporting and identify only its provider, scope, and approximate exposure window.

Please give maintainers reasonable time to investigate and coordinate a fix before public disclosure. Maintainers will acknowledge reports, assess scope, work on a fix or mitigation, credit reporters if they want credit, and publish an advisory or release notes when it is safe to do so. The project does not promise a particular timeline or outcome.

## What this tool sends and stores

HarnessForge is a local CLI, but some commands deliberately make outbound requests:

| Feature | Destination | Data sent |
| --- | --- | --- |
| `ask`, `rag`, and `discover propose` | Selected AI provider | Prompt and, when requested, selected repository excerpts or retrieved chunks; provider/model metadata and normal HTTP request metadata. |
| `embedding create` | OpenAI embeddings endpoint | The supplied input text and selected model. |
| `search` | No network request in the built-in lexical index | The query is evaluated locally against `.harness/index.json`. |
| `rag` | Selected AI provider | Query, prompt, and selected retrieved chunks used to generate the answer. |
| `github learn` | GitHub API | Repository identifier and bounded GET requests for issues and pull-request discussion data; `GITHUB_TOKEN` is sent as HTTP authentication when configured. |
| `plugin run` | The plugin executable and any services it contacts | JSON input supplied to the plugin. HarnessForge cannot constrain the plugin's own network behavior. |

The exact provider endpoint depends on the selected provider and configuration. Operators must review the provider's retention, training, logging, regional-processing, and contractual terms, and obtain organizational approval before sending confidential material. HarnessForge neither negotiates those terms nor verifies a provider's retention settings.

Local artifacts can also be confidential. Treat `.harness/index.json`, Harness IR files, generated `AGENTS.md`/`CLAUDE.md` and skills, prompts, selected context, reports, command output, logs, caches, temporary files, and shell history as potentially sensitive. The local index is plaintext JSON. Do not assume a filename filter, output limit, or citation check is complete secret detection or data-loss prevention.

## Credentials and BYOK

Use short-lived, least-privilege credentials from a process-scoped environment or an approved secret manager. Rotate and revoke credentials promptly after suspected exposure, personnel changes, or reduced need. Prefer a dedicated provider project/account with spend and access limits.

Never place credentials in command arguments, shell history, committed or shared `.env` files, Harness YAML, plugin manifests, source code, generated instructions or reports, test fixtures, screenshots, issue/PR text, logs, caches, or copied terminal output. Environment variables reduce accidental persistence in project files, but are not a secret vault: they can be inherited by child processes, exposed by system inspection or debugging, captured by crash/reporting tools, or copied into logs by other software. `.env` files are not loaded by HarnessForge.

The CLI reads provider credentials at runtime and does not intentionally write them to its preferences or Harness IR. That is not a guarantee that a credential can never appear in provider errors, external logs, local artifacts, or a plugin-accessible file; inspect and protect those channels separately.

## Plugins and generated instructions

`plugin discover` does not execute a plugin. `plugin run ... --authorize` does execute one. `--authorize`, manifest validation, a reduced inherited environment, timeouts, and message limits are application controls; they are **not** OS sandboxing. An authorized plugin is unrestricted local program code with the permissions of the account that launches it. It may read accessible files, make network connections, modify files, or start additional processes. A timeout does not prove that its entire process tree was terminated.

For enterprise use, allow only reviewed, pinned plugin artifacts. Execute them under independently configured OS or container isolation and a dedicated low-privilege identity, with explicit filesystem mounts, network egress policy, resource limits, audit logging, and verified process-tree termination. Review each plugin manifest, binary/source, dependency change, and every generated command, quality gate, or agent instruction before approval or execution. HarnessForge does not enforce those controls and does not determine whether generated guidance is safe or correct.

## Known limitations

- The tool does not provide encryption at rest, a credential vault, operating-system sandboxing, multi-tenant isolation, or complete prevention of secret disclosure.
- Repository selection and context filtering are bounded heuristics, not a complete confidential-data classifier. Review the exact material before enabling provider-backed commands.
- Third-party services, shell tools, CI systems, operating systems, and plugins have their own logging, retention, and access behavior outside HarnessForge's control.
- Security controls evolve with releases. Check this policy and the release notes before deploying an update.

## Incident handling

If you suspect a HarnessForge-related security incident, stop the affected workflow, preserve only the minimum safe evidence, revoke or rotate involved credentials, and use the private reporting route above. Maintainers will triage the report, determine affected versions and mitigations, coordinate disclosure where appropriate, and publish a corrected release or advisory when ready. Operators remain responsible for their own provider accounts, repositories, machines, CI logs, and regulatory or contractual notifications.
