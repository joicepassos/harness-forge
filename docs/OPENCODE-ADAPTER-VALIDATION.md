# OpenCode adapter validation (T8.2)

Status: documentation-based contract with narrow V1 and V2 runtime
validation added on 2026-09-28. Official documentation was consulted on
2026-09-27; the V1 Rules page reported last updated 2026-09-26, while the V2
pages identify a documentation surface but do not specify an OpenCode
release/build number for these semantics. “V1” and “V2” below label
documentation contracts, not pinned binary versions. Runtime evidence is
recorded separately and applies only to the exact tested V2 package and cases.

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
| A | Root `AGENTS.md`; run from root | Root instruction is included | Pass on V1 1.18.33 and V2 2.0.18 |
| B | Root `AGENTS.md` plus `packages/api/AGENTS.md`; run from `packages/api` | V1 upward-discovery and V2 ancestor inclusion | Pass on V1 1.18.33 and V2 2.0.18 |
| C | Root `AGENTS.md` plus `packages/api/AGENTS.md`; start at root, then read/list `packages/api` | V2 nested rule is discovered as area is explored | Pass on V2 2.0.18 |
| D | Only project `CLAUDE.md` | V1 fallback loads; V2 does not use this fallback | V1 fallback passes on 1.18.33; V2 negative case passes on 2.0.18 |
| E | Project `AGENTS.md` and `CLAUDE.md`, contradictory sentinels | V1 chooses AGENTS over CLAUDE; V2 AGENTS-only contract | V1 precedence passes on 1.18.33; V2 case passes on 2.0.18 |
| F | Global OpenCode AGENTS plus global Claude CLAUDE, then remove one at a time | V1 chooses OpenCode global over Claude global; V2 only claims OpenCode AGENTS | V1 precedence and fallback pass on 1.18.33; V2 global Claude comparison pending |
| G | `OPENCODE_DISABLE_PROJECT_CONFIG=1` with project and global AGENTS | V2 omits project, retains global | Pass on V2 2.0.18 |
| H | V1 `opencode.json` `instructions` local file, glob and URL, each isolated | Entries contribute to V1 context; remote timeout is bounded by documented 5 sec | Path, glob, and URL inclusion pass on 1.18.33; timeout duration unmeasured |
| I | V2 `opencode.json` `instructions` local file, glob and URL | Current docs state resolver does not add these entries to model context | All three negative cases pass on V2 2.0.18 |
| J | Workspace outside project root with global AGENTS and parent AGENTS above an inner Git root | The V2 docs say global only; parent sentinel must be absent from instruction entries | Required V2 test; see observed runtime result below |

## Runtime validation: OpenCode V1 1.18.33

The V1 cases were executed on 2026-09-28 on Windows (`win32`) with the npm
package `opencode-ai@1.18.33`; `opencode --version` returned `1.18.33`. The npm
tarball shasum is `1195faeb9b33cb39ad58fc01be813307d1e3b935` and its integrity is
`sha512-58P1ffLRXiAY4QeoABqUjqf6DEzm1ihpovvn9Ad+oOLbLiwK/rf88wruT9IATJ7AtNjmi8iA7pC8tN4o5y799Q==`.
The V1 rules are documented at [OpenCode Rules](https://opencode.ai/docs/rules/);
the custom provider uses the [documented provider configuration](https://opencode.ai/docs/providers/).

Each run used the OpenCode CLI against a local OpenAI-compatible mock bound to
`127.0.0.1:8765`. Temporary XDG config/data/state/cache directories and a
temporary Windows user profile isolated the global OpenCode and Claude files;
the normal profile was not modified. Captured JSONL request bodies were
inspected in the system-message role for unique sentinels. There were 11 CLI
runs and 22 captured requests, including OpenCode's title-generation requests.
The mock returned only `LOCAL_MOCK_OK`; no external model provider was called.

| Case | Observed model-visible instructions | Result |
| --- | --- | --- |
| A | Global `AGENTS.md` and project-root `AGENTS.md`; nested sentinel absent when starting at root | Pass |
| B | Global, project-root, and nested `AGENTS.md` when starting in the nested directory | Pass |
| D | Project `CLAUDE.md` loads when that project has no `AGENTS.md` | Pass |
| E | Project `AGENTS.md` loads; conflicting project `CLAUDE.md` is absent | Pass |
| F | With both global files, OpenCode global `AGENTS.md` loads and global Claude `CLAUDE.md` does not; after removing only the OpenCode file, global Claude fallback loads | Pass |
| H | Local `instructions` path, glob, and loopback URL each add their unique sentinel; the URL mock records `GET /instructions.md` | Pass for inclusion; timeout duration not measured |

Evidence fingerprints: request bodies
`9e3f6db1eaa32680ba0a644a0e1339e1e74b4b2ba87cb4c652af9f40e981d7bc`;
URL access log `e84f03d113e02557e886e8e3f06f7899315e8d6e23c727c8f164ebf16bae1f5f`;
mock server source `71b49173363f5f162a749d0c945e403c287c2e73721fa25bd705e17f31eb0928`.
Fixture hashes: project root AGENTS
`e2033aa72c4c8b9eed8bd97c122eb352839df9c319f00d8362a422e698898338`, nested
AGENTS `0715d17a932244beeedc5fa214c2954280dfbdc3b095f3d0cd75a71adb1ab0b8`,
Claude-only project file
`d780e1ed12b9f713f41a6c9247b8b09ade47da2969fdc279d87685bc59d957ec`, and
isolated global Claude file
`89e9da62c06537dc3bffb72c0ce77980c6e835c0dc391cd967c8d40ec82141bb`.

This evidence applies only to V1 `1.18.33` and the listed cases. The documented
five-second timeout was not measured; V1 disable controls and actual
HarnessForge-published outputs remain unverified. The captured files and mock
were kept under ignored `.pilot-runs/` and were not committed.

## Runtime validation: OpenCode V2 2.0.18

The instruction-discovery cases below were executed on 2026-09-28 using
`@opencode/cli@2.0.18` (`opencode v2.0.18`) on Windows (`win32`). The package
was run from the npm cache. A temporary XDG config home supplied the global
`AGENTS.md` and a custom OpenAI-compatible provider pointing only to a mock
HTTP server on `127.0.0.1`; no external model provider or paid API was used.
The mock recorded each request body as JSONL, so the assertions inspect the
actual messages sent by OpenCode, not filesystem presence or the mock response.
The fixture was a temporary Git repository with distinct sentinels
in the global, root, and `packages/api` instruction files.

Case J used a separate inner Git repository at
`%LOCALAPPDATA%/Temp/opencode/forge-v2-boundary/workspace`, with a parent
`AGENTS.md` one directory above that Git root. Its SHA-256 was
`46b46225a2e289da876cd86d97d2bd380ae5eee0319eeb014851a47e7a53b1a2`.
The first capture was invalid for asserting the sentinel because the query
contained it. Case J was then rerun with a neutral query and evaluated by the
role and instruction-entry content in the captured request JSONL.

The instruction-file SHA-256 values were global
`68eca69381c6d1c7d5d7cdddcf4b225755f37ed0675b50cfcca65208a19870c8`, project
root `00df5bc6ec20187ec5cb7da1c6d051ee18dc3ccf0fa380773899602e57efddf8`, and
`packages/api`
`20246c2275a9200b7b0323bb8e610d09e98ea87e0513b45560e420abdc415fbb`.
The V2 config `instructions` negative-case file hash was
`b38fd1208acffcb36a897428696bf1ac69bd8cec720839cb2a9f26661292ecef`.
The glob file hash was
`5c0de3773a406a148bd8cca25893fa94e8c4c92f5cefd38bea54f0aabe8e0dff`.
The V2 `CLAUDE.md`-only sentinel hash was
`f0f4b8694d95795c8150d4851b86cf07af2253d0a6703a074fc1eaf4b866ff64`; the
conflicting-case `AGENTS.md` and `CLAUDE.md` hashes were
`a3a2065d4a5460256d407065ba264b8112fc424b090e3559cc7e7149baed0ece` and
`d6a78a18f76228931f2cf420475d359dbdfc9ebe64363cb4dbde0667d88f9b04`.
Each run used `opencode run --standalone --model mock/test --format json` from
the listed working directory. OpenCode's JSONL request bodies were checked
across message roles for sentinel text; the returned `LOCAL_MOCK_OK` response
provided no evidence about model instruction-following.

| Case | Run location / setting | Observed model-visible instructions | Result |
| --- | --- | --- | --- |
| A | Project root | Global, project root | Pass |
| B | `packages/api` | Global, `packages/api`, project root, in that order | Pass |
| C | Project root, then successfully read `packages/api/probe.txt` | Nested rule absent initially; `packages/api/AGENTS.md` appears in the next request's instruction entry | Pass |
| D | Project with only `CLAUDE.md` | Global present; project CLAUDE sentinel absent | Pass |
| E | Project with conflicting `AGENTS.md` and `CLAUDE.md` | AGENTS sentinel present; CLAUDE sentinel absent | Pass |
| G | Project root with `OPENCODE_DISABLE_PROJECT_CONFIG=1` | Global only | Pass |
| I | Global config `instructions` points to a local file, glob, and loopback URL, each with a unique sentinel | All three sentinels are absent from captured request messages; the loopback URL was not fetched | Pass |
| J | Inner Git workspace beneath an external directory containing a parent `AGENTS.md`; isolated global config; neutral query | Parent `AGENTS.md` sentinel appears in the system instruction message, despite being outside the inner Git root | Fail against documented “global only” behavior |

For case C, the recorded first request did not contain the nested sentinel. The
mock then issued a controlled `read` call for `packages/api/probe.txt`; after
that successful read, the next request contained an instruction entry sourced
from `packages/api/AGENTS.md`. This confirms dynamic discovery for this exact
build and read path. Case I's local path, glob and URL cases all pass as
negative tests for this exact V2 build. Case J's initial capture was invalid,
but the neutral-query rerun shows the parent `AGENTS.md` is included in system
instructions outside the inner Git root. This conflicts with the current V2
documentation statement that only global instructions load in this setup.
The result does not establish V1 behavior or cross-version `CLAUDE.md` fallback parity,
conflict resolution, or any Forge adapter publication behavior. The mock
returned a fixed local response; this validates effective request context only,
not model compliance or task quality.

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

This artifact remains a research and validation record for T8.2, not evidence
that the OpenCode adapter is implemented. The runtime evidence above covers
effective V2 instruction context for only the named build and seven cases;
other discovery, compatibility, and adapter claims remain open until tested
with their exact release, install channel, OS, fixture revision, and observed
outcome.
