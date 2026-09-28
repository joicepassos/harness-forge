# Canonical Forge source for the no-Forge clone fixture

This reviewed source is compiled by `TestWithoutForgeClonesPreserveExportedContract`.
The Codex and Claude directories beside it contain only the native instruction
and skill files generated for each target, plus the consuming Go project.
Regenerate those native files with `harnessforge sync --repository <copy> --apply`
after reviewing changes to this source. Do not copy `.forge/` into the consumer
clones.
