param(
    [string]$ReportPath = "docs/pilot/mili-context-selection-v1.json",
    [switch]$RequireRawCaptures,
    [switch]$RequireCleanRunnerArtifacts
)

$ErrorActionPreference = "Stop"
$repositoryRoot = (Resolve-Path (Join-Path $PSScriptRoot "../..")).Path
$resolvedReport = Join-Path $repositoryRoot $ReportPath
$report = Get-Content -LiteralPath $resolvedReport -Raw | ConvertFrom-Json
$sidecarPath = Join-Path $repositoryRoot "docs/pilot/mili-context-selection-provenance-v1.json"
$sidecar = Get-Content -LiteralPath $sidecarPath -Raw | ConvertFrom-Json
$failures = [System.Collections.Generic.List[string]]::new()
$warnings = [System.Collections.Generic.List[string]]::new()

function Assert-Equal([string]$Name, $Expected, $Actual) {
    if ($Expected -is [System.Array] -or $Actual -is [System.Array]) {
        $expectedText = @($Expected) -join "`n"
        $actualText = @($Actual) -join "`n"
        if ($expectedText -cne $actualText) {
            $script:failures.Add("$Name`: expected [$expectedText], got [$actualText]")
        }
        return
    }
    if ($Expected -is [string] -or $Actual -is [string]) {
        if ([string]$Expected -cne [string]$Actual) {
            $script:failures.Add("$Name`: expected [$Expected], got [$Actual]")
        }
        return
    }
    if ($Expected -ne $Actual) {
        $script:failures.Add("$Name`: expected [$Expected], got [$Actual]")
    }
}

function Get-Sha256Text([string]$Text) {
    $sha = [System.Security.Cryptography.SHA256]::Create()
    try {
        $bytes = [System.Text.Encoding]::UTF8.GetBytes($Text)
        return ([System.BitConverter]::ToString($sha.ComputeHash($bytes))).Replace("-", "").ToLowerInvariant()
    }
    finally {
        $sha.Dispose()
    }
}

function Get-RepoPath([string]$RelativePath) {
    return Join-Path $repositoryRoot ($RelativePath -replace '/', [IO.Path]::DirectorySeparatorChar)
}

function Normalize-TaskText([string]$Text) {
    $plainText = $Text.Replace('`', '')
    return ([regex]::Replace($plainText, '\s+', ' ')).Trim()
}

function Get-TreeManifestHash([string]$Directory) {
    $root = (Resolve-Path -LiteralPath $Directory).Path
    $records = [System.Collections.Generic.List[string]]::new()
    $files = @(Get-ChildItem -LiteralPath $root -File -Recurse -Force)
    foreach ($file in $files) {
        if (($file.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0) {
            throw "Snapshot contains a reparse-point file: $($file.FullName)"
        }
        $relative = [IO.Path]::GetRelativePath($root, $file.FullName).Replace('\', '/')
        $hash = (Get-FileHash -LiteralPath $file.FullName -Algorithm SHA256).Hash.ToLowerInvariant()
        $records.Add("$hash`t$($file.Length)`t$relative")
    }
    $ordered = [string[]]$records.ToArray()
    [Array]::Sort($ordered, [StringComparer]::Ordinal)
    $manifest = ($ordered -join "`n") + "`n"
    $sha = [System.Security.Cryptography.SHA256]::Create()
    try {
        $bytes = [System.Text.Encoding]::UTF8.GetBytes($manifest)
        $treeHash = ([System.BitConverter]::ToString($sha.ComputeHash($bytes))).Replace('-', '').ToLowerInvariant()
    }
    finally {
        $sha.Dispose()
    }
    return [pscustomobject]@{
        Hash = $treeHash
        FileCount = $files.Count
        TotalBytes = ($files | Measure-Object -Property Length -Sum).Sum
    }
}

Assert-Equal "schema_version" 1 $report.schema_version
Assert-Equal "report_id" "mili-context-selection-v1" $report.report_id
Assert-Equal "metric scope" "offline paired retrieval-selection proxy; not an agent task run" $report.status
Assert-Equal "Mili pinned commit" "49402ad341a7e8a01aee7968d1b10d91b44c0d1c" $report.source_repository.commit
Assert-Equal "HarnessForge command revision" "c97b44213079c0545024c308bdf2838a9e17a82d" $report.runner.harnessforge_commit
Assert-Equal "estimator" "payload-byte-upper-bound-v1" $report.runner.estimator
Assert-Equal "budget" 131072 $report.runner.budget_tokens
Assert-Equal "provider not called" $false $report.runner.external_model_provider_called
Assert-Equal "task set" "MLI-01,MLI-02,MLI-03,MLI-04,MLI-05" (($report.cases | ForEach-Object { $_.task_id }) -join ",")
Assert-Equal "aggregate case count" 5 $report.aggregate.cases

Assert-Equal "provenance sidecar schema" 1 $sidecar.schema_version
Assert-Equal "provenance sidecar ID" "mili-context-selection-provenance-v1" $sidecar.artifact_id
Assert-Equal "provenance classification" "reproducible_declared_provenance_not_signed_execution_attestation" $sidecar.provenance_strength.classification
Assert-Equal "sidecar report path" $ReportPath $sidecar.report.path
Assert-Equal "sidecar report ID" $report.report_id $sidecar.report.report_id
$reportHash = (Get-FileHash -LiteralPath $resolvedReport -Algorithm SHA256).Hash.ToLowerInvariant()
Assert-Equal "sidecar report SHA-256" $sidecar.report.sha256 $reportHash
Assert-Equal "sidecar source commit" $report.source_repository.commit $sidecar.inputs.source_repository.commit
Assert-Equal "sidecar task corpus SHA-256" $report.task_corpus.sha256 $sidecar.inputs.task_corpus.sha256
Assert-Equal "sidecar task IDs" (($report.task_corpus.task_ids) -join ",") (($sidecar.inputs.task_corpus.task_ids) -join ",")
Assert-Equal "sidecar Forge manifest SHA-256" $report.forge_evaluation_source.sha256 $sidecar.inputs.forge.manifest_sha256
Assert-Equal "sidecar Forge knowledge SHA-256" $report.forge_evaluation_source.knowledge_sha256 $sidecar.inputs.forge.knowledge_sha256
Assert-Equal "sidecar clean source commit" $report.runner.harnessforge_commit $sidecar.clean_reproduction.runner_source_commit
Assert-Equal "sidecar original runner revision" $report.runner.harnessforge_commit $sidecar.original_report_runner.build_vcs_revision
Assert-Equal "original binary dirty state retained" $true $sidecar.original_report_runner.build_vcs_modified
Assert-Equal "clean rerun output equality claim" $true $sidecar.clean_reproduction.cases_byte_identical_to_report
Assert-Equal "clean build network policy" "disabled (GOPROXY=off, GOSUMDB=off, GOTOOLCHAIN=local)" $sidecar.clean_reproduction.network_access
Assert-Equal "clean binary build mode" "disabled; source is pinned by the Git archive commit and digest" $sidecar.clean_reproduction.build_vcs_mode

$taskSourcePath = Get-RepoPath "docs/pilot/tasks-draft-v1.md"
$freezePath = Get-RepoPath "docs/pilot/mili-condition-freeze-v2.json"
$manifestPath = Get-RepoPath $report.forge_evaluation_source.manifest
$knowledgePath = Get-RepoPath $report.forge_evaluation_source.knowledge_file
foreach ($item in @(
    @{ Name = "task corpus"; Path = $taskSourcePath; Hash = $report.task_corpus.sha256 },
    @{ Name = "freeze"; Path = $freezePath; Hash = $null },
    @{ Name = "Forge manifest"; Path = $manifestPath; Hash = $report.forge_evaluation_source.sha256 },
    @{ Name = "Forge knowledge"; Path = $knowledgePath; Hash = $report.forge_evaluation_source.knowledge_sha256 }
)) {
    if (-not (Test-Path -LiteralPath $item.Path -PathType Leaf)) {
        $failures.Add("$($item.Name) source file is missing: $($item.Path)")
        continue
    }
    if ($null -ne $item.Hash) {
        $actualHash = (Get-FileHash -LiteralPath $item.Path -Algorithm SHA256).Hash.ToLowerInvariant()
        Assert-Equal "$($item.Name) SHA-256" $item.Hash $actualHash
    }
}

if (Test-Path -LiteralPath $freezePath -PathType Leaf) {
    $freeze = Get-Content -LiteralPath $freezePath -Raw | ConvertFrom-Json
    $snapshotPath = Join-Path (Split-Path -Parent $freezePath) $freeze.approval_record.path
    Assert-Equal "freeze task corpus SHA-256" $report.task_corpus.sha256 $freeze.repository.task_text_source_sha256
    Assert-Equal "freeze approved task IDs" "MLI-01,MLI-02,MLI-03,MLI-04,MLI-05" ($freeze.repository.approved_task_ids -join ",")
    if (-not (Test-Path -LiteralPath $snapshotPath -PathType Leaf)) {
        $failures.Add("frozen approval snapshot is missing: $snapshotPath")
    }
    else {
        $snapshotHash = (Get-FileHash -LiteralPath $snapshotPath -Algorithm SHA256).Hash.ToLowerInvariant()
        Assert-Equal "frozen approval snapshot SHA-256" $freeze.approval_record.sha256 $snapshotHash
    }
}

$taskText = Get-Content -LiteralPath $taskSourcePath -Raw
$taskBodies = @{}
$taskPattern = '(?ms)^\d+\.\s+\*\*(MLI-\d+)[^\r\n]*\*\*\s*\r?\n(?<body>.*?)(?=^\d+\.\s+\*\*MLI-|^#{2,3}\s|\z)'
foreach ($match in [regex]::Matches($taskText, $taskPattern)) {
    $taskBodies[$match.Groups[1].Value] = Normalize-TaskText $match.Groups["body"].Value
}

$taskIds = @("MLI-01", "MLI-02", "MLI-03", "MLI-04", "MLI-05")
$sumBaseline = 0
$sumForge = 0
$sumDelta = 0
$overflowCount = 0
$selectedKnowledgeCount = 0
$rawCaptureCount = 0
$cleanCaptureCount = 0
Assert-Equal "sidecar case count" $report.cases.Count $sidecar.cases.Count
$snapshotPath = Get-RepoPath $sidecar.inputs.source_repository.snapshot_path
if (Test-Path -LiteralPath $snapshotPath -PathType Container) {
    $snapshotManifest = Get-TreeManifestHash $snapshotPath
    Assert-Equal "Mili snapshot tree SHA-256" $sidecar.inputs.source_repository.tree_manifest_sha256 $snapshotManifest.Hash
    Assert-Equal "Mili snapshot file count" $sidecar.inputs.source_repository.file_count $snapshotManifest.FileCount
    Assert-Equal "Mili snapshot byte count" $sidecar.inputs.source_repository.total_bytes $snapshotManifest.TotalBytes
}
elseif ($RequireCleanRunnerArtifacts) {
    $failures.Add("Mili input snapshot is missing: $snapshotPath")
}
else {
    $warnings.Add("Mili input snapshot unavailable; snapshot tree hash not rechecked: $snapshotPath")
}

$sidecarTaskIds = @($sidecar.cases | ForEach-Object { $_.task_id })
Assert-Equal "sidecar task order" (($report.cases | ForEach-Object { $_.task_id }) -join ",") ($sidecarTaskIds -join ",")
foreach ($case in $report.cases) {
    $taskId = [string]$case.task_id
    $run = @($sidecar.cases | Where-Object { $_.task_id -ceq $taskId }) | Select-Object -First 1
    if ($null -eq $run) {
        $failures.Add("sidecar run is missing for $taskId")
        continue
    }
    Assert-Equal "$taskId sidecar prompt" $case.prompt $run.prompt
    Assert-Equal "$taskId sidecar prompt SHA-256" $case.prompt_sha256 $run.prompt_sha256
    Assert-Equal "$taskId sidecar capture path" $case.local_capture_path $run.report_capture.path
    Assert-Equal "$taskId sidecar report capture SHA-256" $case.capture_sha256 $run.report_capture.sha256
    Assert-Equal "$taskId sidecar clean capture equality claim" $true $run.clean_reproduction_capture.byte_identical_to_report_capture
    Assert-Equal "$taskId sidecar clean argv template" "context,explain,$($sidecar.inputs.source_repository.snapshot_path),{prompt},--model,gpt-6-luna,--budget,131072,--compare-knowledge" ($run.argv_template -join ",")
    Assert-Equal "$taskId sidecar clean model" "gpt-6-luna" $run.command_flags.model
    Assert-Equal "$taskId sidecar clean budget" 131072 $run.command_flags.budget
    Assert-Equal "$taskId sidecar clean compare flag" $true $run.command_flags.compare_knowledge
    Assert-Equal "$taskId sidecar clean BM25 flag" $false $run.command_flags.bm25
    Assert-Equal "$taskId sidecar clean MMR flag" $false $run.command_flags.mmr
    Assert-Equal "$taskId sidecar clean task paths" 0 @($run.command_flags.task_paths).Count
    if (-not $taskBodies.ContainsKey($taskId)) {
        $failures.Add("approved task prose not found for $taskId")
    }
    elseif ((Normalize-TaskText $case.prompt) -cne $taskBodies[$taskId]) {
        $failures.Add("$taskId prompt does not match the approved task prose")
    }

    $promptHash = Get-Sha256Text ([string]$case.prompt)
    Assert-Equal "$taskId prompt SHA-256" $case.prompt_sha256 $promptHash
    Assert-Equal "$taskId estimator pairing" $true $case.same_estimator
    Assert-Equal "$taskId selected knowledge ID" $report.forge_evaluation_source.knowledge_id (($case.selected_knowledge_ids) -join ",")

    foreach ($condition in @("baseline_without_knowledge", "with_approved_knowledge")) {
        $plan = $case.$condition
        if ($plan.budget_overflow) { $overflowCount++ }
        $paths = @($plan.included_sources)
        if ($paths.Count -ne [int]$plan.selected_source_count) {
            $failures.Add("$taskId $condition selected source count does not match its path list")
        }
        $uniquePaths = @($paths | Sort-Object -Unique)
        if ($uniquePaths.Count -ne $paths.Count) {
            $failures.Add("$taskId $condition included source paths contain duplicates")
        }
    }

    if (-not $case.baseline_without_knowledge.budget_overflow -and -not $case.with_approved_knowledge.budget_overflow) {
        $expectedDelta = [int]$case.with_approved_knowledge.estimated_tokens - [int]$case.baseline_without_knowledge.estimated_tokens
        Assert-Equal "$taskId reported token delta" $expectedDelta ([int]$case.estimated_token_delta)
    }
    if ($case.selected_knowledge_ids -contains $report.forge_evaluation_source.knowledge_id) { $selectedKnowledgeCount++ }
    $sumBaseline += [int]$case.baseline_without_knowledge.estimated_tokens
    $sumForge += [int]$case.with_approved_knowledge.estimated_tokens
    $sumDelta += [int]$case.estimated_token_delta

    $rawPath = Get-RepoPath $case.local_capture_path
    if (-not (Test-Path -LiteralPath $rawPath -PathType Leaf)) {
        $message = "$taskId raw capture missing: $($case.local_capture_path)"
        if ($RequireRawCaptures) { $failures.Add($message) } else { $warnings.Add($message) }
        continue
    }
    $rawCaptureCount++
    $raw = Get-Content -LiteralPath $rawPath -Raw | ConvertFrom-Json
    $rawHash = (Get-FileHash -LiteralPath $rawPath -Algorithm SHA256).Hash.ToLowerInvariant()
    Assert-Equal "$taskId capture SHA-256" $case.capture_sha256 $rawHash
    foreach ($pair in @(
        @{ Name = "baseline_without_knowledge"; Summary = $case.baseline_without_knowledge },
        @{ Name = "with_approved_knowledge"; Summary = $case.with_approved_knowledge }
    )) {
        $rawPlan = $raw.($pair.Name)
        Assert-Equal "$taskId raw $($pair.Name) budget" $report.runner.budget_tokens $rawPlan.budget_tokens
        Assert-Equal "$taskId raw $($pair.Name) estimator" $report.runner.estimator $rawPlan.estimator
        Assert-Equal "$taskId raw $($pair.Name) estimated tokens" $pair.Summary.estimated_tokens $rawPlan.estimated_tokens
        $rawPaths = @($rawPlan.included | ForEach-Object { if ($_.path) { $_.path } else { $_.source } })
        Assert-Equal "$taskId raw $($pair.Name) included paths" $pair.Summary.included_sources $rawPaths
    }
    $rawSelectedIds = @($raw.with_approved_knowledge.included | Where-Object { $_.knowledge_id } | ForEach-Object { $_.knowledge_id } | Sort-Object -Unique)
    Assert-Equal "$taskId raw selected knowledge ID" $case.selected_knowledge_ids $rawSelectedIds
    $rawDelta = [int]$raw.with_approved_knowledge.estimated_tokens - [int]$raw.baseline_without_knowledge.estimated_tokens
    Assert-Equal "$taskId raw token delta" $case.estimated_token_delta $rawDelta

    $cleanCapturePath = Get-RepoPath $run.clean_reproduction_capture.path
    if (-not (Test-Path -LiteralPath $cleanCapturePath -PathType Leaf)) {
        $message = "$taskId clean reproduction capture missing: $($run.clean_reproduction_capture.path)"
        if ($RequireCleanRunnerArtifacts) { $failures.Add($message) } else { $warnings.Add($message) }
    }
    else {
        $cleanCaptureCount++
        $cleanCaptureHash = (Get-FileHash -LiteralPath $cleanCapturePath -Algorithm SHA256).Hash.ToLowerInvariant()
        Assert-Equal "$taskId clean reproduction capture SHA-256" $run.clean_reproduction_capture.sha256 $cleanCaptureHash
        Assert-Equal "$taskId original/reproduction capture SHA-256" $case.capture_sha256 $cleanCaptureHash
    }
}

foreach ($artifact in @(
    @{ Name = "clean HarnessForge source archive"; Path = $sidecar.clean_reproduction.runner_source_archive_path; Hash = $sidecar.clean_reproduction.runner_source_archive_sha256 },
    @{ Name = "clean HarnessForge binary"; Path = $sidecar.clean_reproduction.binary_path; Hash = $sidecar.clean_reproduction.binary_sha256 },
    @{ Name = "original HarnessForge binary"; Path = $sidecar.original_report_runner.binary_path; Hash = $sidecar.original_report_runner.binary_sha256 }
)) {
    $artifactPath = Get-RepoPath $artifact.Path
    if (-not (Test-Path -LiteralPath $artifactPath -PathType Leaf)) {
        $message = "$($artifact.Name) unavailable: $($artifact.Path)"
        if ($RequireCleanRunnerArtifacts) { $failures.Add($message) } else { $warnings.Add($message) }
        continue
    }
    $artifactHash = (Get-FileHash -LiteralPath $artifactPath -Algorithm SHA256).Hash.ToLowerInvariant()
    Assert-Equal "$($artifact.Name) SHA-256" $artifact.Hash $artifactHash
}

Assert-Equal "aggregate overflow count" $overflowCount $report.aggregate.cases_with_budget_overflow
Assert-Equal "aggregate selected knowledge count" $selectedKnowledgeCount $report.aggregate.cases_selecting_approved_knowledge
Assert-Equal "aggregate mean baseline tokens" ([math]::Round($sumBaseline / $report.cases.Count, 1)) $report.aggregate.mean_baseline_estimated_tokens
Assert-Equal "aggregate mean Forge tokens" ([math]::Round($sumForge / $report.cases.Count, 1)) $report.aggregate.mean_with_approved_knowledge_estimated_tokens
Assert-Equal "aggregate mean token delta" ([math]::Round($sumDelta / $report.cases.Count, 1)) $report.aggregate.mean_estimated_token_delta

Write-Output "Report: $resolvedReport"
Write-Output "Raw captures verified: $rawCaptureCount / $($report.cases.Count)"
Write-Output "Clean rerun captures verified: $cleanCaptureCount / $($report.cases.Count)"
foreach ($warning in $warnings) { Write-Warning $warning }
if ($failures.Count -gt 0) {
    foreach ($failure in $failures) { Write-Error $failure -ErrorAction Continue }
    throw "Context-selection integrity validation failed ($($failures.Count) issue(s))."
}
Write-Output "Integrity checks passed. No relevance or task-quality score was calculated."
