# Project conventions

Write source identifiers, comments, documentation, examples, fixtures, commit messages, and release notes in English. English is the default CLI language. Non-English strings belong only in explicit localization catalogs and localization tests; selecting a locale must not translate machine-readable field names, identifiers, or repository evidence.

Keep domain rules independent of CLI, filesystem, HTTP, and provider implementations. Application services depend on small ports; infrastructure adapters implement variable behavior. Preserve manual Harness IR content and require explicit review before approving generated instructions.

Use synthetic fixtures or an authorized local repository for validation. Do not commit credentials, private paths, external test-project names, or conversation links. Keep credentials in the process environment. Avoid source comments except necessary public API documentation.

Before committing an issue, run `go test ./...`, `go vet ./...`, and `git diff --check`. Exercise meaningful behavior and failure boundaries, including read-only smoke tests for repository readers. Record the relevant validation when closing the issue.
