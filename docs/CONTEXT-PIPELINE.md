# Context pipeline

Repository analysis and context selection share one metadata snapshot:

```text
repository.Scan
  -> RepositorySnapshot
      -> analyzer.AnalyzeSnapshot
      -> metadata ranking
      -> bounded deep reads
      -> context selection and serialized budget
```

The scanner applies repository-wide limits, ignore rules, symlink checks,
file classification, and workspace inference. Manifest directories such as
`frontend/` are treated as workspaces in addition to conventional monorepo
roots. Consumers must not introduce a second repository walk for the same
command.

## Selection policy

1. Enumerate admissible file metadata without reading every file.
2. Rank paths and candidates against the query.
3. Apply the deep-read limit after ranking.
4. Read selected files with bounded, root-relative reads.
5. Filter binary, sensitive, secret-like, and agent-instruction content.
6. Deduplicate and compress excerpts while preserving provenance.
7. Serialize sources and enforce the context budget.

Snapshots own an `os.Root` handle for their lifetime. Callers must close the
snapshot after analysis/context construction; reads after close are rejected.
This keeps Windows file handles short-lived and makes root confinement a
filesystem primitive rather than only a path-checking convention.

The default ranking remains the existing deterministic lexical heuristic.
`context explain --bm25` enables the experimental BM25 baseline for comparison.
`context explain --mmr` enables an experimental diversity-aware ordering that
penalizes lexical overlap between selected excerpts. It is opt-in and should
be compared against the retrieval eval corpus before becoming a default.

The document index exposes the same progression through `search`: the default is
embedding ranking, `--hybrid` blends lexical overlap with embedding similarity,
and `--diverse` applies deterministic MMR-like reordering to hybrid results.
These scores are retrieval baselines and do not establish semantic correctness.
The same flags are available on `rag`; the selected strategy is applied before
citations are sent to the answer provider.

Local budget estimates use `payload-byte-upper-bound-v1`. This is deliberately
named as an upper-bound approximation, not as an exact model tokenizer. A
provider-specific `TokenCounter` can replace it when a model-aware counter is
available and its outbound-data tradeoff is accepted. `Budget.Model` is passed
to the counter so provider/model-specific tokenization can be selected without
coupling the context domain to a provider package.

## Provenance and review

Sources have stable IDs and can preserve multiple excerpts from one file.
Harness evidence separates `file`, `symbol`, `quote`, and `revision`. Quotes
are verified against the referenced file before a rule is accepted.

Workspace metadata is preserved through analyzer findings, context excerpts,
index chunks, retrieval results, and RAG sources, so downstream consumers do
not need to infer project ownership from path strings.

AI-generated rules are persisted as `candidate`; applying them requires an
explicit human approval transition.
