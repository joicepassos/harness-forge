# Codex adapter validation (T4.2)

## Scope contract

Codex loads `AGENTS.md` instructions from the repository root down the current
working directory chain. More specific directory instructions are appended to
the ancestor instructions; `AGENTS.override.md` takes precedence over the
regular file at the same directory. Codex does not provide arbitrary
file-pattern matching for `AGENTS.md`.

HarnessForge therefore emits native nested `AGENTS.md` files only for literal
directory subtree scopes (`directory/**`). A repository-wide `**` scope stays
in the root `AGENTS.md`. The consumer must run Codex with its current working
directory inside the intended subtree for the nested file to load. Other globs,
including file globs and mixed native/non-native scope lists, stay in the root
file as advisory text. The compiler never widens those globs into directory
instructions. Ancestor files accumulate; HarnessForge does not claim semantic
conflict resolution or policy enforcement.

## Local runtime check

On 2026-09-28, `codex-cli 0.158.0-alpha.2.1` ran `codex debug prompt-input`
against a temporary Git repository on Windows 11. The command renders the
model-visible input locally; no model request was made. Three instruction files
contained unique sentinels:

| Working directory | Effective instruction entry | Result |
| --- | --- | --- |
| Repository root | Root sentinel only | Pass |
| `services/api` | Root, `services`, and `services/api` sentinels, in ancestor-to-leaf order | Pass |

The fixture file SHA-256 values were root
`0c8692bd70ab1920ec7b7f2e88092cab383806dae07593ddc05ddddf99133be4`,
`services/AGENTS.md`
`45b3bffa637df6aa141a25c596babf0fc95156f41116c778ad23f0a11ee03f6`, and
`services/api/AGENTS.md`
`8b5b19f612684547df665f2b7229a6f39064c97c98366e57b876df03a3843a00`.
Both prompt-input captures exited successfully and contained an
`agents_md.instructions` entry. The fixture was isolated from the HarnessForge
repository.

This verifies directory-chain discovery for this exact Codex build. It does
not verify model compliance, file-glob enforcement, or behavior in other Codex
versions. Automated renderer and sync tests cover deterministic subtree
compilation, advisory fallback for unsupported globs, ownership, conflict
protection, and symlink safety.

Reference: [Codex custom instructions](https://developers.openai.com/codex/guides/agents-md).
