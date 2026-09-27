# Portable Agent Skills (T8.4)

Forge skills use the open Agent Skills directory format: each skill is a
directory with a `SKILL.md` containing YAML frontmatter (`name` and
`description`) followed by concise instructions. Optional `scripts/`, `references/`
and `assets/` directories are loaded only when the consuming agent supports
them. A Forge manifest reference records a stable ID, description, and
repository-relative directory or file path.

Agent Skills are portable references, but loading is agent-specific. Forge
validates standard `name` and `description` frontmatter, requires the name to
match the skill directory, and publishes the source bundle to
`.agents/skills/<name>/` for Codex and `.claude/skills/<name>/` for Claude Code.
It copies `SKILL.md` and the optional `scripts/`, `references/`, and `assets/`
trees without flattening them. Other top-level resources, symlinks, and special
files are rejected. AGENTS.md and CLAUDE.md contain only a reference to the
published skill, never its instruction body. Generated-manifest ownership
protects updates and removes only intact files from a previously published
bundle. Review skill contents like code; a skill is guidance, not a sandbox or
permission boundary.

Compatibility is target- and version-specific. Codex and Claude Code discovery
must be recorded with exact versions in validation results. Forge preserves
resource directories in the native skill bundle paths; actual client discovery
and resource execution still require version-specific runtime validation.
