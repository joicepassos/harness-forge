# Localization coverage

Brazilian Portuguese (`pt-BR`) and Spanish (`es`) translate common command names, help headings, flags, and shared messages. The catalog deliberately falls back to English when a command description or message has no translation entry. This is partial localization, not a claim that every command is translated.

The behavior is covered by `cmd/harnessforge/localization_test.go`. Check the common paths and a fallback with:

```sh
go test ./cmd/harnessforge -run Localization
go run ./cmd/harnessforge --language pt-BR index --help
```

The latter may include English command text when the catalog has no corresponding entry; that is the documented contract. Add a catalog entry and test whenever expanding translated coverage.
