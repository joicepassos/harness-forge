# Quality gate environment

Quality gates run only when explicitly requested with
`harnessforge check --run-gates`. Each gate may optionally declare environment
overrides in `.harness/harness.yaml` or `.forge/forge.yaml`:

```yaml
quality_gates:
  - id: tests
    command: go test ./...
    env:
      GOFLAGS: -count=1
```

HarnessForge starts each gate with the invoking process environment, then
overrides matching variable names with the gate's `env` values. Other inherited
variables remain visible to the gate. Environment names must match
`[A-Za-z_][A-Za-z0-9_]*`, and values cannot contain NUL.

Keep credentials and other secrets in the invoking shell or a secret manager;
never put them in versioned gate configuration. An `env` map is configuration,
not a secret store.
