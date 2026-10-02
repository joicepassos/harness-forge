# HarnessForge maintenance checklist

Use this checklist when changing repository analysis, context selection, or
Harness IR generation.

## Every pull request

- Run `gofmt` and `go mod tidy -diff`.
- Run `go test ./...` and `go vet ./...`.
- Keep repository traversal behind `internal/repository` unless a change
  explicitly documents a different boundary.
- Keep repository reads bounded and root-relative.
- Add or update a fixture when scanner, analyzer, or context behavior changes.
- Ensure findings expose typed evidence strength and do not manufacture
  confidence values.
- Ensure AI-generated rules and skills remain reviewable proposals until a
  human approval transition is recorded.
- Ensure evidence quotes, symbols, paths, and revisions use their distinct
  fields.

## Security and dependency checks

- Run `govulncheck` for dependency changes.
- Keep GitHub Actions pinned to reviewed commit SHAs.
- Review outbound context changes for ignore rules, sensitive paths, and
  sensitive content handling.

## Retrieval and context changes

- Verify the deep-read limit is applied after metadata ranking, not during the
  filesystem walk.
- Verify the selected context remains within its serialized budget.
- Preserve source IDs and line provenance when multiple excerpts share a file.
- Update retrieval evaluation fixtures when ranking behavior changes.

## Release checks

- Run clean installer smoke tests on Linux, macOS, and Windows.
- Run the exact final-binary security scan.
- Verify Harness IR schema compatibility and migration behavior.
- Review the outbound-context policy and update `SECURITY.md` when needed.

## Performance baselines

The repository includes non-gating benchmarks for the shared pipeline:

- `BenchmarkScanRepository_1K` and `BenchmarkScanRepository_10K`
- `BenchmarkAnalyzeRepository_10K`
- `BenchmarkBuildContext_10K`

Run them with `go test -run '^$' -bench . -benchmem` in the relevant package.
Compare results on the same machine and Go version; do not use absolute wall
time as a shared CI failure threshold.
