# Portable Agent Skills (T8.4)

Forge skills use the open Agent Skills directory format: each skill is a
directory with a `SKILL.md` containing YAML frontmatter (`name` and
`description`) followed by concise instructions. Optional `scripts/`, `references/`
and `assets/` directories are loaded only when the consuming agent supports
them. A Forge manifest reference records a stable ID, description, and
repository-relative directory or file path.

Agent Skills are portable references, but loading is agent-specific. Forge
manifest references preserve their source path for future target-aware sync.
The current Codex/Claude static adapters retain a link to the skill; they do not
currently materialize a standard `.agents/skills/<name>/SKILL.md` bundle. Do not
claim skill installation support until that target path and discovery behavior
are implemented and tested. Review skill contents like code; a skill is
guidance, not a sandbox or permission boundary. Avoid repeating the same skill
body in AGENTS.md and CLAUDE.md; use one shared SKILL.md when the target supports
progressive loading.

Compatibility is target- and version-specific. Codex and Claude Code discovery
must be recorded with exact versions in validation results. Unsupported nested
resource behavior is reported as a limitation, not silently flattened into
static instructions.
