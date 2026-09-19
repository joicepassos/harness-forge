# HarnessForge npm installer

Install HarnessForge with:

```sh
npm install -g harnessforge
```

The package is a thin launcher. During installation, it downloads the matching binary from the official HarnessForge GitHub Release and verifies that archive against the release SHA-256 checksum before extracting it. It does not include the Go application source or provider credentials.

Supported targets are macOS (`amd64`, `arm64`), Linux (`amd64`, `arm64`), and Windows (`amd64`). Node.js 18 or newer and `tar` are required. Modern supported Windows includes `tar.exe`.

Run `harnessforge version` after installation. To update, run `npm update -g harnessforge`. To remove it, run `npm uninstall -g harnessforge`.

The installer performs a network download as an npm lifecycle script. Review this repository and the release artifacts before installation. For direct archive installation, checksums, and security details, see the [main project documentation](https://github.com/joicepassos/harness-forge/tree/main/docs).
