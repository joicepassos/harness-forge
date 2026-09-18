# Synthetic retrieval/RAG evaluation corpus

This corpus is intentionally small, synthetic, and safe to commit. Its source IDs describe fictional backend and frontend documents; it contains no credentials, private URLs, or external project names.

`dataset.yaml` has a related multi-source question and an unrelated-document question. `results.yaml` records a deterministic candidate run. `baseline-report.json` represents a direct-question/no-retrieval baseline, while `candidate-report.json` records the retrieval-grounded comparison. All inputs and report metadata are versioned (`dataset_version`, index, model, prompt, and rubric).

Reproduce the per-case and aggregate candidate report, then compare it with the direct baseline:

```sh
go run ./cmd/harnessforge eval run examples/evals/dataset.yaml examples/evals/results.yaml
go run ./cmd/harnessforge eval compare examples/evals/baseline-report.json examples/evals/candidate-report.json
```

The metrics prove exact-ID retrieval overlap, required-term coverage, citation membership, tokens, latency, and recorded failures. They do not establish semantic correctness, user usefulness, provider quality, or protection against hallucination; every answer still requires human review.
