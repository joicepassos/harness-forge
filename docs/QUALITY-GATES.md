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

Run gates only for repositories whose commands you trust. A gate is an
arbitrary shell command with the invoking user's permissions; `--run-gates`
does not sandbox it. HarnessForge checks that a declared workspace is inside
the repository and has no symlink components before starting the command.
The operating system resolves the working-directory path again when the
process starts, so a concurrent local writer could replace that directory
between the check and launch. For repositories with untrusted concurrent
writers, run the gate in an isolated checkout or another controlled execution
environment.
