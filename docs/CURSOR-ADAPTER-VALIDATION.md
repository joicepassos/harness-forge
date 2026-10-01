# Cursor adapter validation matrix (T8.1)

Research snapshot: 2026-09-30. This records the current public Cursor documentation and proposes a testable target contract for Forge. The official Rules and CLI pages are rolling and do not give minimum application/CLI versions for most rule features; they are product documentation, not a versioned specification. Therefore “current” below means the behavior stated by the official documentation on the snapshot date, not a compatibility guarantee for any particular installed build. No Cursor editor or CLI runtime is available in this environment; every executable scenario below remains unverified.

## Verified from official documentation

| Capability | Documented behavior | Forge implication |
|---|---|---|
| Project rules location and extension | Project rules are version-controlled `.mdc` files under `.cursor/rules/`. A plain `.md` file in that directory is ignored by the rules system. Rules can be organized into folders inside `.cursor/rules`. | Emit `.cursor/rules/<stable-name>.mdc`; never emit rule content as `.md` under that folder. |
| Rule metadata | MDC frontmatter controls application through `description`, `globs`, and `alwaysApply`. | Keep Forge source semantics explicit and render valid frontmatter. Validate the header and body independently. |
| Always | `alwaysApply: true` makes a rule always included; docs say globs and description are ignored in this mode. | Preserve only the always behavior; report any source scope/description as intentionally inactive or a loss. |
| File-scoped | `alwaysApply: false` with `globs` auto-attaches when a matching file is in context. Multiple patterns can be comma-separated. Docs show `*`, `**`, root and recursive examples. | Convert only supported glob syntax; warn/error for patterns whose semantics Forge cannot demonstrate to match. Do not claim exact equivalence for another glob dialect. |
| Agent-selected | `alwaysApply: false` plus `description` and omitted globs makes the rule available for the Agent to select when relevant. | This is best-effort model selection, not deterministic coverage. Surface that distinction in capability output. |
| Manual | `alwaysApply: false` with no description or globs applies only when @-mentioned. | A Forge “manual” rule may render this shape; verify the generated file name is addressable as expected. |
| Nested rule files | Rules may be arranged in subfolders under `.cursor/rules`; docs say a rule is identified by its full file path, so same basenames can coexist. | Deterministic paths can include a stable ID or sanitized name; no basename-only collision assumptions. No stronger directory-local scope is documented for nested rule folders. |
| AGENTS.md | Current Rules docs say `AGENTS.md` works at the project root and in subdirectories. Nested instructions combine with parent instructions; more specific instructions take precedence. | Root and nested AGENTS output is a portable mechanism for directory scope, but do not merge it with MDC automatically without testing interaction. |
| Team/User precedence | Team rules take precedence over project rules, which take precedence over user rules; applicable instructions are merged, earlier sources take precedence on conflicts. Team rules may be enforced and cannot be disabled by team members. | Forge can describe only repository-generated project guidance. It cannot know account/team rules; diagnostics must not promise final effective precedence. |
| Legacy `.cursorrules` | Current Rules page's listed types no longer includes `.cursorrules`; the legacy file is absent from its current documented rule format. Older official docs described it as supported but deprecated. | Treat `.cursorrules` as legacy/undocumented for current support. Do not emit it. Import it only as legacy input with an explicit migration warning; test detection and never infer precedence. |
| Cursor CLI | CLI docs state it supports the same `.cursor/rules` system as the editor and additionally reads root `AGENTS.md` and `CLAUDE.md`. The Rules page documents nested `AGENTS.md` for Cursor generally, but the CLI page does not specify nested discovery. | Do not assume nested `AGENTS.md` behavior is shared by CLI; validate editor and CLI separately. Root CLAUDE.md is documented for CLI; nested CLI discovery and full parity remain unverified. |
| Scope of application | Rules docs say rule context affects Agent and that rules do not affect Tab or other AI features; the current FAQ says User Rules do not apply to Inline Edit. | Describe the adapter as Agent-context output; do not claim Tab completion, Bugbot, or all inline-edit coverage. |
| Version declarations | The current Rules and CLI documentation are rolling pages and do not state a minimum Cursor editor build or CLI version for `.mdc`, nested AGENTS.md, or the precedence model. The source snapshot for this matrix is 2026-09-27. | Forge must not invent a minimum version. Store the tested product, exact build/version, OS, date and scenario in validation results. |

Sources are linked in [Official sources](#official-sources). The old documentation's “AGENTS.md root only (v1.5), nested support planned for v1.6” wording is historical and must not be used as the current contract: the current Rules page explicitly states nested AGENTS.md support is available.

## Proposed Forge adapter capability mapping

| Forge concept | Cursor representation | Fidelity / limitation to report |
|---|---|---|
| Always-applied instruction | `.cursor/rules/<id>.mdc`, frontmatter `alwaysApply: true` | Documented mapping. Any source file glob or agent-selection description is not active with this shape. |
| File-scoped instruction | `.cursor/rules/<id>.mdc`, `alwaysApply: false`, `globs: ...` | Documented concept, but glob dialect equivalence is unproven. Requires runtime fixture tests for every supported pattern class. |
| Advisory/agent-selected instruction | `.cursor/rules/<id>.mdc`, `alwaysApply: false`, non-empty `description`, no `globs` | Selection depends on Agent relevance judgment; cannot claim deterministic inclusion. |
| Manually invoked instruction | `.cursor/rules/<id>.mdc`, `alwaysApply: false`, no `description` or `globs` | Documented trigger; manual @-mention discovery/name behavior needs runtime test. |
| Folder/directory scope | `AGENTS.md` at root and relevant subdirectories, or root `.cursor/rules` with globs | The Rules page documents nested AGENTS hierarchy and specificity for Cursor generally; the CLI page explicitly promises only root AGENTS/CLAUDE files. Precedence between AGENTS.md and project rules is not documented. Preserve separate outputs and validate editor/CLI behavior independently. |
| Skill, executable gate, evidence URI, approval state, review provenance | No equivalent established by the rules docs | Keep these in Forge; do not silently flatten into a claim of native Cursor support. If text is rendered into a rule, mark it as copied guidance with loss of execution/governance semantics. |
| Organization/team-wide constraints | Cursor Team Rules in dashboard | Not project-file output, account-controlled and plan-dependent. Forge adapter cannot create or inspect these; mention that external rules may override project output. |

Recommended stable adapter contract: emit only project-owned artifacts (`.cursor/rules/*.mdc` and, only where requested, `AGENTS.md`); do not overwrite user-authored or unowned files; include mapping/loss diagnostics in dry-run; never claim knowledge of user/team rule state. Target label should identify `Cursor Agent (Editor)` or `Cursor CLI`, plus tested exact version/build, rather than an unqualified “Cursor supported”.

## Runtime tests still required (not proven by documentation)

These tests require an installed Cursor build and a disposable repository. On the 2026-09-28 audit, `cursor-agent`, `agent`, and `cursor` were not available on PATH, so no runtime test could be run. Record exact Cursor editor version/build, CLI version (`agent --version` if available), OS, date, workspace root and outcome when a runtime is available. Public docs provide no minimum version baseline, so test at least the currently supported stable build at validation time and the oldest build Forge elects to support; if no oldest build is selected, mark the compatibility range unknown.

The 2026-09-30 Windows recheck also found no Cursor executable or local
installation. Official documentation now provides a native Windows CLI
installer and `agent --version` verification. Cursor's Hobby tier is free,
requires no credit card, and includes limited Agent requests; a real CLI
Agent run still requires Cursor account authentication and available quota.
No installation, login, or runtime request was made in this audit.

| ID | Fixture / action | Evidence to collect |
|---|---|---|
| C-01 | Root `.cursor/rules/rule.mdc` with `alwaysApply: true`; ask Agent a neutral prompt. | Rule visible in Agent context / behavior; exact build and conversation transcript. |
| C-02 | `alwaysApply: false`, matching glob; open a matching file, then non-matching file. | Whether rule enters context in each case; paths and glob syntax. |
| C-03 | Multiple comma-separated globs, `*`, `**`, root-only and recursive patterns. | Per-pattern inclusion results, including Windows paths and case sensitivity. |
| C-04 | Agent-requested rule with description versus the same rule with no description. | Availability/selection behavior; repeat enough runs to show non-determinism rather than treating one selection as a guarantee. |
| C-05 | Manual rule: @-mention by basename and by full relative path; duplicate names under different `.cursor/rules` subdirectories. | Which files are discovered/applied and whether collisions occur. |
| C-06 | `.cursor/rules/sub/rule.mdc` and a nested `.cursor/rules` directory under a source subdirectory. | Establish the actual recursive discovery boundary; current docs only clearly promise folders within `.cursor/rules`, not independent nested `.cursor/rules` roots. |
| C-07 | Root and nested `AGENTS.md` with deliberately conflicting instructions. | Verify documented parent/child combination and specificity in editor and CLI. |
| C-08 | Both AGENTS.md and `.cursor/rules/*.mdc` with conflicting content. | Determine cross-format order/merging; documentation does not specify it. |
| C-09 | Team, project and user rules with conflicts; repeat with enforced and non-enforced Team Rule. | Verify documented precedence in an eligible Team/Enterprise account; record account setup without storing secrets. |
| C-10 | `.cursorrules` only, then `.cursorrules` plus an Always MDC. | Determine whether current builds still load legacy input and any observed precedence; classify as observed compatibility only, not supported contract. |
| C-11 | Same repository in Cursor editor Agent and Cursor CLI Agent. | Compare rule discovery and activation, including nested AGENTS; document separate results. |
| C-12 | Trigger Tab completion, Inline Edit and Bugbot review with an active project rule. | Confirm documented feature boundaries; do not extend claims to features not documented as consuming rules. |

Release gate for T8.1: check in runtime results with the exact versions and all expected-positive/expected-negative fixtures; maintain docs-based claims separately from observed behavior. A successful static render or parser test alone does not satisfy this target-validation task.

## Official sources

- [Cursor Rules documentation](https://cursor.com/docs/rules) — current project-rule formats, metadata behavior, globs, Team/User precedence, nested `AGENTS.md`, feature limitations and legacy context.
- [Cursor CLI: Using Agent in CLI](https://prod.cursor.com/docs/cli/using) — CLI rule system, root `AGENTS.md`/`CLAUDE.md`, and relation to editor rules (checked 2026-09-28).
- [Cursor CLI overview](https://cursor.com/docs/cli/overview) — current CLI install and modes.
- [Cursor CLI installation](https://cursor.com/docs/cli/installation) — native Windows installer and version check (checked 2026-09-30).
- [Cursor pricing](https://cursor.com/pricing) — Hobby tier and limited Agent use (checked 2026-09-30).
- [Cursor changelog 0.45.x](https://cursor.com/changelog/0-45-x) — historical introduction of `.cursor/rules` (January 2025); not a current minimum-version guarantee.
