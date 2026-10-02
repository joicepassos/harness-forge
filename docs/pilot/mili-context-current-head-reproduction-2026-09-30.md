# Mili context capture reproduction on current Forge code

Date: 2026-09-30 (America/Sao_Paulo). This is an offline retrieval-selection
check for T6.4, not a coding-agent run or task-quality measurement. It does not
change the frozen Mili pilot or the historical context report.

## Inputs and runner

- HarnessForge source: clean Git commit
  `71a50ab07201ac5fc1511a754b13dbcaab359446`.
- SHA-256 of `git archive --format=tar HEAD`:
  `1f9c45fa0d50b8b8608136f6301122157076e9d3bfb76cf5a9228d67357b03ee`.
- Build: Go `1.26.2` on Windows/amd64,
  `GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local`,
  `go build -buildvcs=false -o harnessforge-71a50ab.exe ./cmd/harnessforge`.
- SHA-256 of the clean executable:
  `8a9ea011dd139018e10ecb7cdb4a8c9532378e2d8b6df356bc9021d582312d92`.
- Mili source copy: the 198-file frozen snapshot at commit
  `49402ad341a7e8a01aee7968d1b10d91b44c0d1c`, tree-manifest SHA-256
  `315765fd296acd1ddebc94c701a825314899b3fccbaf54a6b099f5fe1ab4c170`.
  The historical [integrity verifier](verify-context-selection.ps1) passed
  with `-RequireRawCaptures -RequireCleanRunnerArtifacts` before this run.
- Prompt text and command arguments came from
  [the historical provenance sidecar](mili-context-selection-provenance-v1.json):
  `context explain .pilot-runs/mili-context-eval-v1/repo <one approved prompt>
  --model gpt-6-luna --budget 131072 --compare-knowledge`. Each prompt was
  passed as one process argument. No ranking options or task paths were added.

## Results

Raw stdout was captured as bytes without PowerShell text redirection. Every
command exited 0 with empty stderr. Each new capture's SHA-256 exactly equals
the corresponding historical `MLI-0x-clean-head.json` capture:

| Task | Matching capture SHA-256 |
| --- | --- |
| MLI-01 | `d89b2d17b9feb1dbf876371607839c41679f9c5ea851cdb44d135601ef586e5f` |
| MLI-02 | `8425d914e34d345615570a160477df3c7c8596803eca44da95513f405e3178d3` |
| MLI-03 | `941356e5736bbfbaadaa95cee1da117d94ba304c102fb1a17bb6d7e94b496913` |
| MLI-04 | `742e0b72d4124a1fb896a0a195cb1327df5b20e0f0cfa9ebe6964d60ca42da55` |
| MLI-05 | `71d385ed5e6c0824efdf14f47b454428bb61657a4f743d66e44e58c4227618ee` |

Thus the same-source excerpt and discovery-payload fixes through this runner
commit do not change these five historical paired selections or token
estimates. The captures had no duplicate selected source keys. The result
still measures a retrieval proxy with a conservative token estimator; it says
nothing about model output, task correctness, human review, or a Forge benefit.
The new raw captures and executable were retained in an isolated temporary
checkout, not added to the versioned report or presented as an attested run.
