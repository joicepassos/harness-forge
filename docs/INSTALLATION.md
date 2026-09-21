# Installation and update security

## Quick installation

With Node.js 18 or newer, run `npm install -g harnessforge`, then `harnessforge version`. Starting with v1.1.0, the first invocation downloads the matching release binary and checks its SHA-256 digest without an npm lifecycle install script. In the target project, run `harnessforge init` to follow the guided setup. If the project is already configured, the command reports its status without changing files.

The direct installers below are useful when npm is unavailable.

HarnessForge distributes only through GitHub Releases. The release workflow publishes versioned archives and a checksum manifest for:

| Platform | Architectures | Installer |
| --- | --- | --- |
| macOS | `amd64`, `arm64` | `install.sh` |
| Linux | `amd64`, `arm64` | `install.sh` |
| Windows | `amd64` | `install.ps1` |

Both installers select the platform and architecture, download the matching archive and checksum manifest over HTTPS, compare the archive's SHA-256 digest with the named manifest entry, reject unexpected archive paths, and install only after verification. A failed checksum is a hard stop.

## macOS and Linux

Download the script to a file, inspect it, and then execute it:

```sh
VERSION=1.1.0
curl --fail --location --proto '=https' --tlsv1.2 \
  "https://github.com/joicepassos/harness-forge/releases/download/v${VERSION}/install.sh" \
  --output install-harnessforge.sh
less install-harnessforge.sh
sh install-harnessforge.sh --version "$VERSION" --install-dir "$HOME/.local/bin"
```

The script accepts `--install-dir` and does not modify `PATH`. Add `$HOME/.local/bin` to the shell configuration yourself if required. The one-line `curl | sh` form is intentionally discouraged because it executes a remote response before inspection. If a one-line form is used in a disposable environment, pin an exact release URL and understand that remote script execution still requires trust in the downloaded script.

## Windows

Download and inspect the PowerShell script before executing it:

```powershell
$Version = '1.1.0'
Invoke-WebRequest "https://github.com/joicepassos/harness-forge/releases/download/v$Version/install.ps1" -OutFile .\install-harnessforge.ps1
Get-Content .\install-harnessforge.ps1
Set-ExecutionPolicy -Scope Process Bypass
.\install-harnessforge.ps1 -Version $Version -InstallDir "$env:USERPROFILE\bin"
```

The Windows installer requires a 64-bit operating system and `tar.exe`, which is included in supported modern Windows installations. It does not modify the system execution policy or `PATH` permanently.

## Manual archive installation

Alternatively, download the archive and its matching `harnessforge_<version>_checksums.txt` asset from the release page. Verify the exact archive line with `sha256sum`/`shasum` on Unix or `Get-FileHash` on PowerShell before extracting. Extract only the expected `harnessforge` or `harnessforge.exe` entry and place it in a directory you control.

## Updates and uninstall

Run the same installer with a newer version and the same destination to update. For rollback, install a previously published version after checking its original checksum manifest. To uninstall, remove only the executable installed by the chosen command and remove its directory from `PATH` if it is no longer used. Do not overwrite release assets or reuse a version for different bytes.

Development builds are not release assets and are not accepted by the installers. The scripts support a local HTTP base URL only for maintainers' isolated tests; normal use always defaults to the official HTTPS GitHub Releases endpoint.
