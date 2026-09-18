# Project conventions

Write source identifiers, comments, documentation, examples, fixtures, commit messages, and release notes in English. English is the default CLI language. Non-English strings belong only in explicit localization catalogs and localization tests; selecting a locale must not translate machine-readable field names, identifiers, or repository evidence.

Keep domain rules independent of CLI, filesystem, HTTP, and provider implementations. Application services depend on small ports; infrastructure adapters implement variable behavior. Preserve manual Harness IR content and require explicit review before approving generated instructions.

Use synthetic fixtures or an authorized local repository for validation. Do not commit credentials, private paths, external test-project names, or conversation links. Keep credentials in the process environment. Avoid source comments except necessary public API documentation.

Before committing an issue, run `go test ./...`, `go vet ./...`, and `git diff --check`. Exercise meaningful behavior and failure boundaries, including read-only smoke tests for repository readers. Record the relevant validation when closing the issue.

## Releases

Contributors, not end users, prepare core releases. Before tagging, run `go test ./...`, `go vet ./...`, `git diff --check`, and cross-build every published target where the local toolchain permits it. This repository is the single source for the Go product and its GitHub Release installers: `install.sh` supports macOS/Linux and `install.ps1` supports Windows amd64. No package manager publication is implied.

Pushing an annotated `v1.*` tag starts the release workflow. It is the only tag pattern allowed to create a release. Review generated archives, SHA-256 checksums, changelog, installer behavior, and the `version` command's injected version, commit, and build date before announcing availability. Test both installers against a known fixture, including an incorrect checksum and an unsupported platform or architecture.

Release archives are built with path stripping, disabled implicit VCS stamping, a commit-derived source date, and explicit linker metadata. These controls make comparisons meaningful but do not replace independent reproduction or cryptographic provenance. The current checksum manifest is unsigned; planned work is signing and build attestation. Do not claim signed artifacts or attestations until they are actually published and verified.

If a release is wrong, never overwrite its tag, archive, checksum, or installer script. Mark it as bad, preserve the evidence, and publish a corrected patch version. Tell users to discard failed downloads and rerun the installer for the corrected version; never replace bytes under the same version. See `docs/INSTALLATION.md` and `docs/RELEASES.md` for installation, rollback, and provenance details.
