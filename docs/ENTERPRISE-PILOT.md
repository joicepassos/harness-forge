# Controlled enterprise-pilot checklist

HarnessForge is suitable only for a limited, supervised pilot until the release gates are independently met. This checklist is an operational control, not a security certification.

## Entry criteria

- The repository owner has approved each pilot repository as trusted and non-production-critical; forks, public issue content, and unreviewed pull requests are excluded from provider-bound workflows.
- The organization has approved the exact AI provider, model, retention terms, and outbound data categories. Run `context explain` before the first provider call and review the selected material.
- Use a dedicated, least-privilege, short-lived credential from an approved secret manager or process-scoped environment. Do not put it in YAML, arguments, `.env` files, logs, generated files, or shared shells. Define an owner and rotation/revocation path.
- Plugins are disabled unless each extension is version-pinned, reviewed, and executed under an independently isolated low-privilege account or OS/container policy. HarnessForge does not sandbox plugins.
- A named technical reviewer owns the Harness IR, generated instruction changes, and any provider-bound output.

## Required checkpoints

1. Before indexing or asking a provider, review repository scope and exclusions, then preview selected context.
2. Before applying a discovery proposal or generating instructions, a human reviews evidence, source changes, and the resulting diff.
3. Before using an answer in a decision, a subject-matter reviewer checks citations against the repository and records the disposition.
4. If credentials, private material, unexpected output, or plugin behavior is suspected, stop provider/plugin work, preserve minimal evidence, revoke or rotate credentials as appropriate, and follow the security policy.

## Exit criteria

- The pilot records its allowed repositories, provider/model versions, reviewer decisions, failures, and evaluation results without storing sensitive prompts or credentials.
- No unresolved boundary, credential, data-flow, or human-review failure remains. Any such failure blocks expansion.
- Pilot owners explicitly decide whether to stop, remediate and repeat, or expand scope. Expansion requires a separate risk review; a successful pilot does not authorize broad release.
