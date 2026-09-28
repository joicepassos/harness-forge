param(
    [string]$ReportPath = "docs/pilot/mili-context-selection-v1.json",
    [switch]$RequireRawCaptures
)

$ErrorActionPreference = "Stop"
$repositoryRoot = (Resolve-Path (Join-Path $PSScriptRoot "../..")).Path
$resolvedReport = Join-Path $repositoryRoot $ReportPath
$report = Get-Content -LiteralPath $resolvedReport -Raw | ConvertFrom-Json
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
foreach ($case in $report.cases) {
    $taskId = [string]$case.task_id
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
}

Assert-Equal "aggregate overflow count" $overflowCount $report.aggregate.cases_with_budget_overflow
Assert-Equal "aggregate selected knowledge count" $selectedKnowledgeCount $report.aggregate.cases_selecting_approved_knowledge
Assert-Equal "aggregate mean baseline tokens" ([math]::Round($sumBaseline / $report.cases.Count, 1)) $report.aggregate.mean_baseline_estimated_tokens
Assert-Equal "aggregate mean Forge tokens" ([math]::Round($sumForge / $report.cases.Count, 1)) $report.aggregate.mean_with_approved_knowledge_estimated_tokens
Assert-Equal "aggregate mean token delta" ([math]::Round($sumDelta / $report.cases.Count, 1)) $report.aggregate.mean_estimated_token_delta

Write-Output "Report: $resolvedReport"
Write-Output "Raw captures verified: $rawCaptureCount / $($report.cases.Count)"
foreach ($warning in $warnings) { Write-Warning $warning }
if ($failures.Count -gt 0) {
    foreach ($failure in $failures) { Write-Error $failure -ErrorAction Continue }
    throw "Context-selection integrity validation failed ($($failures.Count) issue(s))."
}
Write-Output "Integrity checks passed. No relevance or task-quality score was calculated."
