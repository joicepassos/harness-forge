param(
    [string]$ReportPath = (Join-Path $PSScriptRoot 'mili-context-selection-v1.json'),
    [string]$JudgmentsPath = (Join-Path $PSScriptRoot 'mili-source-relevance-judgments-v1.json'),
    [string]$OutputPath = (Join-Path $PSScriptRoot 'mili-relevance-coverage-v1.json')
)

$ErrorActionPreference = 'Stop'

function Get-Sha256([string]$Path) {
    return (Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash.ToLowerInvariant()
}

$reportFullPath = (Resolve-Path -LiteralPath $ReportPath).Path
$judgmentsFullPath = (Resolve-Path -LiteralPath $JudgmentsPath).Path
$report = Get-Content -LiteralPath $reportFullPath -Raw | ConvertFrom-Json
$judgments = Get-Content -LiteralPath $judgmentsFullPath -Raw | ConvertFrom-Json
$repositoryRoot = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
$sidecarPath = Join-Path $repositoryRoot $judgments.source_repository.selection_provenance_sidecar_path
$sidecar = Get-Content -LiteralPath $sidecarPath -Raw | ConvertFrom-Json
$freezePath = Join-Path $repositoryRoot $judgments.frozen_protocol.freeze_path
$rubricPath = Join-Path $repositoryRoot $judgments.frozen_protocol.rubric_path
$freeze = Get-Content -LiteralPath $freezePath -Raw | ConvertFrom-Json
$sourceSnapshotRoot = Join-Path $repositoryRoot $judgments.source_repository.selection_snapshot_path

if ($judgments.status -ne 'provisional_analyst_judgments' -or $judgments.is_ground_truth -ne $false) {
    throw 'Judgments must remain explicitly provisional and not ground truth.'
}
if ($report.source_repository.commit -ne $judgments.source_repository.commit) {
    throw 'Report and judgments do not refer to the same pinned source commit.'
}
if ((Get-Sha256 $reportFullPath) -cne $sidecar.report.sha256) {
    throw 'Paired report hash differs from its provenance sidecar.'
}
if ((Get-Sha256 $sidecarPath) -cne $judgments.source_repository.selection_provenance_sidecar_sha256) {
    throw 'Selection provenance sidecar hash differs from the judgment artifact.'
}
if ($sidecar.inputs.source_repository.tree_manifest_sha256 -cne $judgments.source_repository.selection_snapshot_tree_manifest_sha256) {
    throw 'Pinned source snapshot tree manifest hash differs from the judgment artifact.'
}
if ((Get-Sha256 $freezePath) -cne $judgments.frozen_protocol.freeze_sha256) { throw 'Freeze hash differs from judgment artifact.' }
if ((Get-Sha256 $rubricPath) -cne $judgments.frozen_protocol.rubric_sha256) { throw 'Evaluator rubric hash differs from judgment artifact.' }
if ($judgments.source_repository.commit -cne $sidecar.inputs.source_repository.commit) {
    throw 'Judgments do not refer to the source commit bound in the provenance sidecar.'
}
$sidecarCases = @{}
foreach ($sidecarCase in $sidecar.cases) { $sidecarCases[$sidecarCase.task_id] = $sidecarCase }
foreach ($evaluator in $judgments.frozen_evaluator_sources) {
    $evaluatorPath = Join-Path $repositoryRoot $evaluator.path
    if (-not (Test-Path -LiteralPath $evaluatorPath -PathType Leaf)) {
        throw "Frozen evaluator source is missing: $($evaluator.path)"
    }
    if ((Get-Sha256 $evaluatorPath) -cne $evaluator.sha256) {
        throw "Frozen evaluator source hash differs: $($evaluator.path)"
    }
    $frozenEntry = @($freeze.evaluation.evaluator_sources | Where-Object {
        [System.IO.Path]::GetFileName($_.path) -ceq [System.IO.Path]::GetFileName($evaluator.path)
    }) | Select-Object -First 1
    if ($null -eq $frozenEntry -or $frozenEntry.sha256 -cne $evaluator.sha256) {
        throw "Evaluator source is not bound by freeze v2: $($evaluator.path)"
    }
}

$reportCases = @{}
foreach ($case in $report.cases) { $reportCases[$case.task_id] = $case }
$judgmentCases = @{}
foreach ($task in $judgments.tasks) { $judgmentCases[$task.task_id] = $task }
$taskIds = @($reportCases.Keys | Sort-Object)
if (($taskIds -join ',') -cne (@($judgmentCases.Keys | Sort-Object) -join ',')) {
    throw 'Report and judgments task IDs differ.'
}
if ($taskIds.Count -ne 5 -or $report.cases.Count -ne 5 -or $judgments.tasks.Count -ne 5 -or $sidecarCases.Count -ne 5) {
    throw 'Expected exactly five frozen tasks in the report, judgments, and provenance sidecar.'
}

$uniqueJudgedPaths = @{}
foreach ($judgmentTask in $judgments.tasks) {
    foreach ($judgedPath in $judgmentTask.paths) { $uniqueJudgedPaths[$judgedPath.path] = $judgedPath }
}
foreach ($judgedPath in $uniqueJudgedPaths.Values) {
    if (-not $judgedPath.pinned_head_git_blob_oid -or -not $judgedPath.pinned_head_file_sha256) {
        throw "Pinned source hash missing for $($judgedPath.path)."
    }
    $snapshotFile = Join-Path $sourceSnapshotRoot $judgedPath.path
    if (-not (Test-Path -LiteralPath $snapshotFile -PathType Leaf)) { throw "Pinned source snapshot path missing: $($judgedPath.path)" }
    if ((Get-Sha256 $snapshotFile) -cne $judgedPath.pinned_head_file_sha256) {
        throw "Pinned source SHA-256 differs: $($judgedPath.path)"
    }
    $actualBlobOid = (git hash-object -- $snapshotFile).Trim()
    if ($actualBlobOid -cne $judgedPath.pinned_head_git_blob_oid) {
        throw "Pinned source Git blob differs: $($judgedPath.path)"
    }
}

$conditions = @(
    @{ key = 'baseline_without_knowledge'; id = 'baseline_without_knowledge' },
    @{ key = 'with_approved_knowledge'; id = 'with_approved_knowledge' }
)
$rows = [System.Collections.Generic.List[object]]::new()
$allCovered = 0
$allAnnotated = 0
$macroSum = 0.0
$macroN = 0

foreach ($taskId in $taskIds) {
    $task = $judgmentCases[$taskId]
    if (-not $sidecarCases.ContainsKey($taskId)) { throw "Provenance sidecar lacks $taskId." }
    $case = $reportCases[$taskId]
    $sidecarCase = $sidecarCases[$taskId]
    if ($case.capture_sha256 -cne $sidecarCase.report_capture.sha256 -or
        $case.capture_sha256 -cne $sidecarCase.clean_reproduction_capture.sha256) {
        throw "$taskId report and clean-rerun capture hashes do not match the provenance sidecar."
    }
    foreach ($captureRelativePath in @($case.local_capture_path, $sidecarCase.clean_reproduction_capture.path)) {
        $capturePath = Join-Path $repositoryRoot $captureRelativePath
        if (-not (Test-Path -LiteralPath $capturePath -PathType Leaf)) { throw "Capture missing: $captureRelativePath" }
        if ((Get-Sha256 $capturePath) -cne $case.capture_sha256) { throw "Capture hash mismatch: $captureRelativePath" }
    }
    $relevant = @($task.paths | Where-Object { $_.grade -in @('high', 'medium') } | ForEach-Object { $_.path } | Sort-Object -Unique)
    if ($relevant.Count -eq 0) { throw "$taskId has no high/medium paths." }
    foreach ($condition in $conditions) {
        $plan = $case.($condition.key)
        $included = @($plan.included_sources | Sort-Object -Unique)
        $includedSet = [System.Collections.Generic.HashSet[string]]::new([string[]]$included, [System.StringComparer]::Ordinal)
        $covered = @($relevant | Where-Object { $includedSet.Contains($_) } | Sort-Object -Unique)
        $missing = @($relevant | Where-Object { -not $includedSet.Contains($_) } | Sort-Object -Unique)
        $ratio = [math]::Round($covered.Count / $relevant.Count, 6)
        $rows.Add([pscustomobject]@{
            task_id = $taskId
            condition = $condition.id
            capture_sha256 = $case.capture_sha256
            annotated_high_medium_path_count = $relevant.Count
            covered_path_count = $covered.Count
            coverage = $ratio
            covered_paths = $covered
            missing_annotated_paths = $missing
        })
        $allCovered += $covered.Count
        $allAnnotated += $relevant.Count
        $macroSum += $ratio
        $macroN++
    }
}

$result = [ordered]@{
    schema_version = 1
    report_id = 'mili-relevance-coverage-v1'
    status = 'provisional_annotated_path_coverage_only'
    generated_on = (Get-Date -Format 'yyyy-MM-dd')
    is_ground_truth = $false
    metric = 'annotated_high_medium_path_coverage'
    formula = 'unique selected paths labeled high or medium divided by the number of unique high or medium paths annotated for that task'
    interpretation = 'Coverage of this provisional, non-exhaustive analyst-annotated path set only. Both conditions may already include all annotated candidates; 100% coverage is not evidence of improvement. This is not ground-truth recall, precision, NDCG, source-quality, or task-quality evidence.'
    source_repository = $judgments.source_repository
    inputs = @{
        paired_report = [ordered]@{
            path = [System.IO.Path]::GetRelativePath($repositoryRoot, $reportFullPath).Replace('\', '/')
            sha256 = Get-Sha256 $reportFullPath
        }
        provisional_judgments = [ordered]@{
            path = [System.IO.Path]::GetRelativePath($repositoryRoot, $judgmentsFullPath).Replace('\', '/')
            sha256 = Get-Sha256 $judgmentsFullPath
        }
        provenance_sidecar = [ordered]@{
            path = [System.IO.Path]::GetRelativePath($repositoryRoot, $sidecarPath).Replace('\', '/')
            sha256 = Get-Sha256 $sidecarPath
            source_tree_manifest_sha256 = $sidecar.inputs.source_repository.tree_manifest_sha256
        }
    }
    aggregate = [ordered]@{
        task_condition_pairs = $macroN
        expected_task_condition_pairs = 10
        micro_covered_paths = $allCovered
        micro_annotated_paths = $allAnnotated
        micro_coverage = [math]::Round($allCovered / $allAnnotated, 6)
        macro_mean_task_condition_coverage = [math]::Round($macroSum / $macroN, 6)
        by_condition_macro_mean = @(
            foreach ($condition in $conditions) {
                $conditionRows = @($rows | Where-Object { $_.condition -eq $condition.id })
                [pscustomobject]@{
                    condition = $condition.id
                    task_count = $conditionRows.Count
                    macro_mean_coverage = [math]::Round((($conditionRows | Measure-Object -Property coverage -Average).Average), 6)
                }
            }
        )
    }
    results = @($rows)
    limitations = @(
        'Only paths listed in the provisional judgment artifact are counted; the candidate pool is not exhaustive.',
        'A low or absent annotation is not treated as a definitive irrelevance judgment.',
        'No precision, NDCG, ground-truth recall, agent outcome, or task-quality score is calculated.'
    )
}

$outputDirectory = Split-Path -Parent $OutputPath
if (-not (Test-Path -LiteralPath $outputDirectory -PathType Container)) {
    New-Item -ItemType Directory -Path $outputDirectory -Force | Out-Null
}
$result | ConvertTo-Json -Depth 12 | Set-Content -LiteralPath $OutputPath -Encoding utf8
Write-Output "Wrote provisional path coverage: $OutputPath"
Write-Output ("Macro mean task-condition coverage: {0:P2}" -f $result.aggregate.macro_mean_task_condition_coverage)
Write-Output ("Micro annotated-path coverage: {0:P2} ({1}/{2})" -f $result.aggregate.micro_coverage, $allCovered, $allAnnotated)
