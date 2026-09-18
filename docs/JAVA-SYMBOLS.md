# Java symbol extraction boundary

Date verified: 2026-09-18.

HarnessForge extracts Java type declarations (`class`, `interface`, `record`, and `enum`) and named method declarations from syntactically balanced source. It removes comments and string/character literals before matching, so declaration-shaped text in those regions is not reported. Method symbols use the `method` kind.

This is intentionally a bounded lexer rather than a complete Java compiler front end. It supports ordinary named methods and interface method declarations whose parameter lists do not contain braces or semicolons. It does not claim complete coverage of every Java grammar feature (including annotation-driven declarations, lambdas, compact record constructors, or methods with exotic generic syntax). A source with unterminated comments or literals, unclosed braces, or unexpected closing braces is rejected with a `parse Java source` error. Consumers requiring full Java grammar coverage must use a compiler-backed parser before treating symbol output as authoritative.

The regression fixtures in `internal/symbols/infrastructure/parsers_test.go` cover method extraction, comment/literal exclusion, and syntax-error behavior. Run them with:

```sh
go test ./internal/symbols/infrastructure
```
