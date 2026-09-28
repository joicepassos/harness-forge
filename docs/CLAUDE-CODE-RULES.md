# Claude Code instruction export

The Claude Code adapter writes repository-wide project context and unscoped
approved rules to `CLAUDE.md`. Every approved rule with path scopes is written
to its own `.claude/rules/<stable-hash>.md` file. The file begins with YAML
frontmatter containing the authored `paths` list; the patterns are serialized
with `yaml.v3` and retained literally. A SHA-256-derived filename prevents
rule IDs from becoming unsafe filesystem paths.

Claude Code documents `.claude/rules/*.md` files with a `paths` frontmatter
field as path-scoped rules. See the [official rules and frontmatter reference](https://code.claude.com/docs/en/memory#path-specific-rules).
HarnessForge declares `native-path-frontmatter;glob-parity-unverified` for
those output files: path-scoped loading is native, but the adapter does not
claim that Claude Code's glob dialect matches HarnessForge validation in all
cases. Patterns are never rewritten or widened. `CLAUDE.md` is marked
`global-only` in generated metadata.

The single-document `ClaudeAdapter.Render` API returns an error when scoped
rules require additional output files. `CompileForge` and `sync` use
`RenderDocuments`, so each scoped file participates in dry-run, apply, check,
ownership, stale-file cleanup, and interrupted-apply recovery. The sync path
rejects unsafe names, symlinks, and manually edited owned files.
