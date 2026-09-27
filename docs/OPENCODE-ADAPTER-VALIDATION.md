# OpenCode adapter validation (T8.2)

Status: documentation-based contract proposal. No OpenCode binary was run as
part of this research. Official documentation was consulted on 2026-09-27; the
V1 Rules page reported last updated 2026-09-26, while the V2 pages identify a
documentation surface but do not specify an OpenCode release/build number for
these semantics. “V1” and “V2” below label documentation contracts, not pinned
binary versions. The linked official pages are the source of documented
behavior; installed-version behavior still needs executable tests.

## Verified behavior from official documentation

| Surface | V1 documentation (`/docs/rules/`) | V2 documentation (`/v2/docs/instructions/`) | Adapter consequence |
| --- | --- | --- | --- |
| Project rule filename | `AGENTS.md` in the project applies in that directory and descendants. | `AGENTS.md` is the supported project instruction filename. | Prefer generating root `AGENTS.md` for both documented surfaces. |
| Project fallback | `CLAUDE.md` in the project is used if no `AGENTS.md` exists. Local rule files are found by traversing up from the current directory. | V2 explicitly says it does not use `CLAUDE.md` as fallback. | A `CLAUDE.md`-only export is V1 compatibility behavior, not a V2-safe export. Never emit both as if they were additive. |
| Global fallback | `~/.config/opencode/AGENTS.md` is a global source. `~/.claude/CLAUDE.md` is used if the OpenCode global file does not exist. | Loads `$XDG_CONFIG_HOME/opencode/AGENTS.md` (normally `~/.config/opencode/AGENTS.md`); no Claude fallback is described. | Keep global user instructions outside project-generated ownership. The Forge adapter must not write into a home directory. |
| Precedence / combination | The first matching file wins within a category: `AGENTS.md` wins over `CLAUDE.md`; the OpenCode global file wins over the Claude global file. Local and global sources are listed as separate categories. | Loads the global file, then every project `AGENTS.md` from the current Location up to the project root. The displayed order is global, deepest nested, parent, root. Sources are combined; conflicts are not resolved by OpenCode. | The V1 and V2 contracts differ. Do not promise a single universal “last file wins” rule or collapse hierarchy into one flat precedence rule. |
| Nested project rules | V1 documents upward traversal from the current directory; it does not specify discovery of all deeper rules while exploring. | Nested `AGENTS.md` below the workspace is discovered when the agent reads a file or lists a directory in that area. Newly discovered content is appended in discovery order. | Root export is discoverable at startup; nested scope behavior is version-dependent and must be tested with actual directory reads. |
| Outside project root | V1 rules page identifies project and global categories but does not specify all worktree/outside-root boundary details. | If workspace is outside the project root, only the global file is loaded. | Treat external worktrees and `--directory` roots as an explicit test case; do not infer behavior from normal project-root cases. |
| Disable controls | `OPENCODE_DISABLE_CLAUDE_CODE=1` disables Claude compatibility; `OPENCODE_DISABLE_CLAUDE_CODE_PROMPT=1` disables the global Claude prompt fallback. | `OPENCODE_DISABLE_PROJECT_CONFIG=1` skips project AGENTS discovery without disabling the global file. | Test environment overrides independently; report the resulting output as “not loaded by configuration,” not as an adapter failure. |
| Config `instructions` | V1 supports local paths, globs and remote URLs; all are combined with AGENTS rules. Remote fetch timeout is documented as 5 seconds. | The V2 schema accepts `instructions`, but currently does not resolve files, globs, or URLs into model context. | Do not use V2 `instructions` as a delivery path until a tested release documents/implements resolution. |

Sources: [V1 Rules](https://opencode.ai/docs/rules/) and [V2 Instructions](https://opencode.ai/v2/docs/instructions/). The V1 page states it was last updated 2026-09-26. These docs distinguish V1 and V2 semantics but do not give an exact minimum released version for every AGENTS discovery behavior. Therefore this document does not infer a version boundary from the URL or claim that all installed builds match the current docs.

## Version-qualified capability matrix

`Documented` means directly described by the linked official docs. `Test required`
means the behavior should not be advertised by HarnessForge until exercised
against a pinned OpenCode binary. Record the exact `opencode --version`, install
channel, OS, and test fixture revision with every result.

| Capability | V1 docs contract | V2 docs contract | Current adapter claim | Required executable validation |
| --- | --- | --- | --- | --- |
| Root `AGENTS.md` | Documented | Documented | Safe baseline target | Start OpenCode from project root and a descendant; verify generated marker appears in effective instructions. |
| Project `CLAUDE.md` fallback | Documented only when no project `AGENTS.md` exists | Explicitly not supported as fallback | V1-only compatibility capability | Fixture with only CLAUDE; then both files with conflicting sentinel instructions; run on pinned V1 and V2. |
| Global OpenCode `AGENTS.md` | Documented | Documented | Outside Forge file ownership | Test with a temporary isolated config home and prove it combines/loads as docs say. |
| Global `~/.claude/CLAUDE.md` fallback | Documented if OpenCode global AGENTS absent | No fallback in V2 instructions contract | V1-only compatibility capability | Test with each global file alone and both together, including the disable env vars. |
| Ancestor and nested `AGENTS.md` | Upward traversal documented; exact multi-file conflict semantics are not fully specified | Every ancestor from current Location to root is loaded; deeper files are dynamic on read/list | V1 ordering details pending; V2 behavior documented, still runtime-tested | Distinct sentinel per directory; open/read/list paths in controlled order; inspect effective prompt/debug output. |
| Conflict semantics among project files | First match per category; AGENTS preferred over CLAUDE | Files are combined; OpenCode does not resolve their conflicts | No guarantee that generated nested outputs override root outputs | Same rule with contradictory values at root/nested scopes; capture effective context and report user-visible outcome. |
| `instructions` config paths/globs/URLs | Documented as supported and combined with AGENTS | Schema accepts field, but resolver currently does not load entries | Do not rely on it for V2 | Test local path, glob and URL separately only on the exact supported V1 version; verify no V2 context inclusion claim. |
| Disable project discovery | Not specified on V1 Rules page | `OPENCODE_DISABLE_PROJECT_CONFIG=1` documented | V2-specific | Verify project file absent from effective instructions while global file remains present. |

### Release recording rule

Maintain concrete rows per tested build, for example `V1 x.y.z` and `V2 x.y.z`,
instead of a floating “OpenCode” capability row. At research time the official
V1 and V2 docs are separate documentation surfaces, but they do not bind the
instruction rules above to a minimum patch release. Pin release versions from
the official installation/release channel when tests are added; do not derive
support claims from current documentation alone. OpenCode's migration guide
also advises verifying the installed V2 behavior needed before relying on it:
[Migrate from V1](https://opencode.ai/v2/docs/migrate-v1/).

## Proposed fixture matrix

Each case should use temporary homes/configuration so personal instructions do
not affect results. Prefer observable, unique sentinel text and capture the
effective model context or a supported diagnostic; filesystem existence alone
does not prove that OpenCode loaded a file.

| Case | Files / action | Expected assertion grounded in docs | Status |
| --- | --- | --- | --- |
| A | Root `AGENTS.md`; run from root | Root instruction is included | Required test |
| B | Root `AGENTS.md` plus `packages/api/AGENTS.md`; run from `packages/api` | V1 upward-discovery and V2 ancestor inclusion | Required test per pinned version |
| C | Root `AGENTS.md` plus `packages/api/AGENTS.md`; start at root, then read/list `packages/api` | V2 nested rule is discovered as area is explored | Required V2 test |
| D | Only project `CLAUDE.md` | V1 fallback loads; V2 does not use this fallback | Required test per major version |
| E | Project `AGENTS.md` and `CLAUDE.md`, contradictory sentinels | V1 chooses AGENTS over CLAUDE; V2 AGENTS-only contract | Required test per major version |
| F | Global OpenCode AGENTS plus global Claude CLAUDE, then remove one at a time | V1 chooses OpenCode global over Claude global; V2 only claims OpenCode AGENTS | Required test per major version |
| G | `OPENCODE_DISABLE_PROJECT_CONFIG=1` with project and global AGENTS | V2 omits project, retains global | Required V2 test |
| H | V1 `opencode.json` `instructions` local file, glob and URL, each isolated | Entries contribute to V1 context; remote timeout is bounded by documented 5 sec | Required V1 test |
| I | V2 `opencode.json` `instructions` local file, glob and URL | Current docs state resolver does not add these entries to model context | Required V2 regression/negative test |
| J | Workspace outside project root with global AGENTS and nearby project AGENTS | V2 loads global only | Required V2 test |

## Adapter and test work still required

1. Keep the V1 and V2 semantic profiles separate in the adapter capability
   report. “Supports OpenCode” without a tested version is too broad.
2. For the portable first implementation, generate a single project-root
   `AGENTS.md`. Treat nested exports as a separate capability because V1 and V2
   discover them differently.
3. Define deterministic collision behavior when Forge already owns
   `AGENTS.md`, a user has edited it, or a pre-existing `CLAUDE.md` would be
   selected as V1 fallback. Preserve user bytes and report an explicit conflict.
4. Verify effective context rather than just checking generated file paths.
   Record the binary version, install channel, environment variables, cwd,
   fixture hash, and observed sentinels.
5. Do not claim parity for V2 `instructions` configuration, `CLAUDE.md`
   fallback, or dynamic nested discovery until tests establish the exact
   released-build behavior.

This artifact is a research/validation proposal for T8.2, not evidence that the
OpenCode adapter is implemented or that any executable compatibility test has
passed. The local inventory found no `opencode` executable on PATH, so
discovery, precedence, workspace-boundary, environment-override, and
effective-context claims have not been runtime-validated. Documentation-based
behavior must not be presented as a tested version matrix until the exact
release, install channel, OS, fixture revision, and observed outcome are
recorded.
