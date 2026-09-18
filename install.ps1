[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [ValidatePattern('^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$')]
    [string] $Version,
    [string] $InstallDir = (Join-Path $env:USERPROFILE 'bin')
)

$ErrorActionPreference = 'Stop'
$goos = 'windows'
if (-not [Environment]::Is64BitOperatingSystem) {
    throw 'Unsupported architecture: HarnessForge releases support Windows amd64 only.'
}
$goarch = 'amd64'
$baseUrl = if ($env:HARNESSFORGE_RELEASE_BASE_URL) { $env:HARNESSFORGE_RELEASE_BASE_URL.TrimEnd('/') } else { 'https://github.com/joicepassos/harness-forge/releases/download' }
if ($baseUrl -notmatch '^https://|^http://(localhost|127\.0\.0\.1):') {
    throw 'Release base URL must use HTTPS.'
}

$archive = "harnessforge_${Version}_${goos}_${goarch}.tar.gz"
$checksums = "harnessforge_${Version}_checksums.txt"
$releaseUrl = "$baseUrl/v$Version"
$temporaryRoot = Join-Path ([IO.Path]::GetTempPath()) ("harnessforge-install-" + [Guid]::NewGuid().ToString('N'))
$archivePath = Join-Path $temporaryRoot $archive
$checksumsPath = Join-Path $temporaryRoot $checksums
$extractDir = Join-Path $temporaryRoot 'extracted'

try {
    New-Item -ItemType Directory -Path $extractDir -Force | Out-Null
    Invoke-WebRequest -Uri "$releaseUrl/$checksums" -OutFile $checksumsPath -UseBasicParsing
    Invoke-WebRequest -Uri "$releaseUrl/$archive" -OutFile $archivePath -UseBasicParsing
    if ((Get-Item -LiteralPath $checksumsPath).Length -gt 1MB) { throw 'Checksum manifest exceeds the supported size limit.' }
    if ((Get-Item -LiteralPath $archivePath).Length -gt 64MB) { throw 'Release archive exceeds the supported download size limit.' }

    $pattern = '^([0-9a-fA-F]{64})\s+\*?' + [regex]::Escape($archive) + '$'
    $line = Get-Content -LiteralPath $checksumsPath | Where-Object { $_ -match $pattern } | Select-Object -First 1
    if (-not $line -or $line -notmatch '^([0-9a-fA-F]{64})') {
        throw "Checksum manifest does not contain $archive."
    }
    $expected = $Matches[1].ToLowerInvariant()
    $actual = (Get-FileHash -LiteralPath $archivePath -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($actual -ne $expected) {
        throw "Checksum mismatch for $archive; refusing to install."
    }

    $entries = @(& tar.exe -tzf $archivePath)
    if ($LASTEXITCODE -ne 0) { throw 'Could not inspect the verified release archive.' }
    $entries = @($entries | Where-Object { $_ })
    if ($entries.Count -ne 1 -or $entries[0] -ne 'harnessforge.exe') { throw 'Release archive contains unexpected paths.' }
    & tar.exe -xzf $archivePath -C $extractDir harnessforge.exe
    if ($LASTEXITCODE -ne 0) { throw 'Could not extract the verified release archive.' }
    $source = Join-Path $extractDir 'harnessforge.exe'
    if (-not (Test-Path -LiteralPath $source -PathType Leaf) -or ((Get-Item -LiteralPath $source).Attributes -band [IO.FileAttributes]::ReparsePoint)) { throw 'Release archive has no regular harnessforge.exe executable.' }

    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    $destination = Join-Path $InstallDir 'harnessforge.exe'
    $staged = Join-Path $InstallDir ('.harnessforge-' + [Guid]::NewGuid().ToString('N') + '.tmp')
    Copy-Item -LiteralPath $source -Destination $staged -Force
    Move-Item -LiteralPath $staged -Destination $destination -Force
    Write-Output "Installed HarnessForge $Version to $destination"
}
finally {
    if ($staged -and (Test-Path -LiteralPath $staged)) { Remove-Item -LiteralPath $staged -Force -ErrorAction SilentlyContinue }
    if (Test-Path -LiteralPath $temporaryRoot) { Remove-Item -LiteralPath $temporaryRoot -Recurse -Force -ErrorAction SilentlyContinue }
}
