<p align="center">
  <img src="assets/branding/harnessforge-logo-v1.png" width="180" alt="HarnessForge logo">
</p>

<h1 align="center">HarnessForge</h1>

<p align="center">
  Build trustworthy AI workflows for your codebase.
</p>

<p align="center">
  <a href="https://github.com/joicepassos/harness-forge/actions/workflows/ci.yml"><img src="https://github.com/joicepassos/harness-forge/actions/workflows/ci.yml/badge.svg" alt="Checks"></a>
  <a href="https://github.com/joicepassos/harness-forge/releases"><img src="https://img.shields.io/github/v/release/joicepassos/harness-forge?display_name=tag" alt="Latest release"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="MIT License"></a>
</p>

HarnessForge is a CLI that understands your repository, turns reviewed rules into agent instructions, and keeps AI answers grounded in evidence you can inspect.

## Install

Download a pinned release and install it. HarnessForge verifies the downloaded archive against its SHA-256 checksum before extraction.

```sh
curl -fsSLO https://github.com/joicepassos/harness-forge/releases/download/v1.0.2/install.sh
sh install.sh --version 1.0.2 --install-dir "$HOME/.local/bin"
```

For Windows, use the inspected PowerShell installer:

```powershell
$Version = '1.0.2'
Invoke-WebRequest "https://github.com/joicepassos/harness-forge/releases/download/v$Version/install.ps1" -OutFile .\install-harnessforge.ps1
Get-Content .\install-harnessforge.ps1
.\install-harnessforge.ps1 -Version $Version -InstallDir "$env:USERPROFILE\bin"
```

Release binaries support macOS and Linux (`amd64`, `arm64`) and Windows (`amd64`). See [Installation](docs/INSTALLATION.md) for manual downloads, updates, and troubleshooting.

## Get started

```sh
# Understand a repository without changing it.
harnessforge analyze --git --format json /path/to/project

# Create a reviewable project harness.
cd /path/to/project
harnessforge init
harnessforge validate

# Generate agent instructions after review.
harnessforge generate codex
```

HarnessForge stores approved rules in `.harness/harness.yaml` and produces reproducible `AGENTS.md` or `CLAUDE.md` files.

## Why HarnessForge?

| | |
| --- | --- |
| **Understand before changing** | Analyze languages, conventions, Git metadata, files, and symbols without modifying the project. |
| **Review the rules** | Keep agent guidance in ordinary YAML that your team can approve in code review. |
| **Use AI with evidence** | Select bounded context, retrieve sources, and reject answers with unsupported citations. |

## Safe by design

- Provider keys are read at runtime and are never written to HarnessForge preferences or harness files.
- Sensitive paths, symlinks, binary files, and secret-like content are excluded from repository context.
- Plugins require explicit `--authorize` permission and do not receive provider credentials by default.

Read the [Security policy](SECURITY.md) before connecting a provider or executing a plugin.

## Learn more

- [Usage guide](docs/USAGE.md) — workflows, examples, and command map.
- [Installation](docs/INSTALLATION.md) — verified installers and checksums.
- [Security policy](SECURITY.md) — BYOK, data flow, and plugin boundaries.
- [Contributing](CONTRIBUTING.md) — develop and contribute.

HarnessForge is open source under the [MIT License](LICENSE).
