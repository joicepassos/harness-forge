# Release operations

This document separates using a release from producing one. This repository produces immutable GitHub Release archives, checksums, GitHub build attestations, and the checksum-verifying `install.sh` and `install.ps1` installers. Contributors should follow the review and release procedure in [CONTRIBUTING.md](../CONTRIBUTING.md). Keep credentials out of the repository and in protected GitHub Environments or the relevant account settings.

## Release-builder policy

The release workflows use Go 1.26.2, a currently supported patched toolchain selected independently from the `go 1.23` module compatibility directive. The fixed version is reviewed after every Go security release; a maintainer updates the workflow, runs the full native CI matrix, and records the change in the pull request before accepting it. Release tooling is fixed in `.tool-versions` and every GitHub Action is referenced by an immutable commit ID with the reviewed tag in a comment. Dependency updates must change both the fixed reference and this review record, never silently follow a major tag or version range.

## Published platforms

Tagged `v1.*` releases build these target pairs:

| Operating system | Architectures |
| --- | --- |
| macOS | `arm64`, `amd64` |
| Linux | `arm64`, `amd64` |
| Windows | `amd64` |

Every archive has a corresponding line in `harnessforge_<version>_checksums.txt`. The archive names use `harnessforge_<version>_<os>_<arch>.tar.gz`; Windows archives contain `harnessforge.exe`.

## Release procedure

1. Start from a reviewed commit with a clean working tree and run the required Go checks from the contributing guide using Go 1.26.2.
2. Choose an unused semantic version and review the generated release notes and archive names.
3. Create and push an annotated `v1.*` tag. The release workflow runs only for that tag pattern. It runs tests, vetting, and source vulnerability checks natively on Linux, macOS, and Windows; creates a candidate archive set; and performs clean installer smoke tests on each operating system before publication.
4. Confirm the release's version, commit, build date, checksum manifest, archive contents, release-binary vulnerability scan, installer scripts, and build attestations before announcing it.

The core workflow creates the authoritative GitHub Release. No npm, Homebrew, or Scoop publication is part of the initial distribution plan. Users install from the release page or inspect and run the matching installer described in [Installation](INSTALLATION.md).

The core release job targets the GitHub Environment named `release`. Configure it with required reviewers before enabling releases, and protect `v1.*` tags so only release maintainers can create them. These are organization settings and are intentionally not emulated by repository files.

## Rollback and correction

Never replace an archive, checksum, tag, or installer script in place. A changed artifact under the same name breaks verifiability and any client that has already checked it. For a release defect, mark the GitHub release as a known-bad release with a clear notice, stop any pending announcement, and issue a new patch version with a new tag and checksum manifest. Tell users to discard failed downloads and install the corrected version; do not republish different bytes under the same version.

Keep the original release record and document the impact, replacement version, and recovery instructions. A security incident requires the project's normal incident process and may require revoking signing material once signing is enabled.

## Provenance and reproducibility

The release workflow checks out the tagged commit with full history, derives `SOURCE_DATE_EPOCH` from that commit, builds with `-trimpath` and `-buildvcs=false`, and injects the Git tag, commit, and commit date into the CLI through linker flags. Archive names and SHA-256 manifests are deterministic functions of the GoReleaser release inputs.

This is a reproducibility-oriented configuration, not a claim that every byte has been independently reproduced. Contributors can compare a local cross-build from the same Go toolchain and source revision with the published checksums. Differences in toolchain, archive implementation, or release metadata can still affect bytes and must be investigated rather than ignored.

SHA-256 protects distribution-channel consumers against corrupted or substituted archive bytes when the official checksum manifest is trusted. Each archive and checksum manifest is also submitted to GitHub's artifact-attestation service from the protected release workflow. Before announcing a release, verify both the digest and provenance locally with the GitHub CLI:

```sh
gh release download vVERSION --repo joicepassos/harness-forge \
  --pattern 'harnessforge_VERSION_checksums.txt' \
  --pattern 'harnessforge_VERSION_linux_amd64.tar.gz'
sha256sum --check --ignore-missing harnessforge_VERSION_checksums.txt
gh attestation verify harnessforge_VERSION_linux_amd64.tar.gz \
  --repo joicepassos/harness-forge \
  --signer-repo joicepassos/harness-forge
```

Replace `VERSION` with the exact published version without a leading `v`, and choose the archive for the platform under review. `gh attestation verify` must identify the HarnessForge repository as the signer and must succeed for the downloaded bytes. The CLI verifier validates the Sigstore bundle and GitHub OIDC identity; a checksum alone is not equivalent provenance.

## Required GitHub controls

Workflow YAML cannot enforce account-level protections. Before enabling releases, repository administrators must separately verify that the default branch requires the pull-request checks, `v1.*` tag creation is limited to release maintainers, the `release` environment requires reviewers, and only the release workflow receives `contents: write`, `attestations: write`, and `id-token: write`. Record that review in the release PR or change request; do not treat the presence of this file as proof that those controls are active.
