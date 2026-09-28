package analyzer

import (
	"context"
	"crypto/sha256"
	"fmt"
	"harnessforge/internal/repository"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

type Analysis struct {
	Project        string        `json:"project"`
	Languages      []Finding     `json:"languages,omitempty"`
	Build          []Finding     `json:"build,omitempty"`
	Frameworks     []Finding     `json:"frameworks,omitempty"`
	Infrastructure []Finding     `json:"infrastructure,omitempty"`
	Database       []Finding     `json:"database,omitempty"`
	Tests          []Finding     `json:"tests,omitempty"`
	Git            []Finding     `json:"git,omitempty"`
	QualityGates   []QualityGate `json:"quality_gates,omitempty"`
	GoModules      []GoModule    `json:"go_modules,omitempty"`
	Files          int           `json:"files"`
}

type GoModule struct {
	Path      string `json:"path"`
	Module    string `json:"module"`
	Workspace string `json:"workspace,omitempty"`
}

type QualityGate struct {
	ID         string   `json:"id"`
	Command    string   `json:"command"`
	Reason     string   `json:"reason"`
	Workspace  string   `json:"workspace,omitempty"`
	Workspaces []string `json:"workspaces,omitempty"`
}

type Finding struct {
	Value string `json:"value"`
	// Confidence is retained for wire compatibility; Strength is the
	// calibrated-looking field consumers should use until detector-specific
	// calibration exists.
	Confidence    float64        `json:"confidence,omitempty"`
	Strength      string         `json:"strength"`
	Evidence      []string       `json:"evidence,omitempty"`
	EvidenceItems []EvidenceItem `json:"evidence_items,omitempty"`
	EvidenceCount int            `json:"evidence_count,omitempty"`
	Workspace     string         `json:"workspace,omitempty"`
	Workspaces    []string       `json:"workspaces,omitempty"`
}

type EvidenceItem struct {
	Path      string `json:"path"`
	Kind      string `json:"kind,omitempty"`
	Workspace string `json:"workspace,omitempty"`
	SHA256    string `json:"sha256,omitempty"`
	Quote     string `json:"quote,omitempty"`
	StartLine int    `json:"start_line,omitempty"`
	EndLine   int    `json:"end_line,omitempty"`
}

type Repository struct {
	Path     string
	Files    []string
	Snapshot *repository.RepositorySnapshot
}

type Options struct {
	IncludeGit bool
}

func Analyze(repositoryPath string) (*Analysis, error) {
	return AnalyzeWithOptions(context.Background(), repositoryPath, Options{})
}

func AnalyzeWithOptions(ctx context.Context, repositoryPath string, options Options) (*Analysis, error) {
	absolutePath, err := filepath.Abs(repositoryPath)
	if err != nil {
		return nil, err
	}
	info, err := os.Lstat(absolutePath)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("repository must be a non-symlink directory")
	}

	snapshot, err := repository.Scan(ctx, absolutePath, repository.ScanOptions{HonorIgnores: true, SkipDirs: repository.DefaultSkipDirs()})
	if err != nil {
		return nil, err
	}
	defer snapshot.Close()
	return AnalyzeSnapshot(ctx, snapshot, options)
}

// AnalyzeSnapshot analyzes an existing inventory without traversing the
// filesystem again.
func AnalyzeSnapshot(ctx context.Context, snapshot *repository.RepositorySnapshot, options Options) (*Analysis, error) {
	if snapshot == nil {
		return nil, fmt.Errorf("repository snapshot must not be nil")
	}
	analysis := &Analysis{Project: filepath.Base(snapshot.Root)}
	files := make([]string, 0, len(snapshot.Files))
	for _, file := range snapshot.Files {
		files = append(files, file.Path)
	}

	repository := Repository{
		Path:     snapshot.Root,
		Files:    files,
		Snapshot: snapshot,
	}

	analysis.Files = len(files)
	results := runDetectors(ctx, repository, options)
	if results.err != nil {
		return nil, results.err
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	analysis.Languages = results.languages
	analysis.Build = results.build
	analysis.Frameworks = results.frameworks
	analysis.Infrastructure = results.infrastructure
	analysis.Database = results.database
	analysis.Tests = results.tests
	analysis.Git = results.git
	annotateWorkspaces(analysis, snapshot)
	analysis.QualityGates = qualityGates(snapshot, analysis)
	analysis.GoModules = goModules(snapshot)

	return analysis, nil
}

func goModules(snapshot *repository.RepositorySnapshot) []GoModule {
	if snapshot == nil {
		return nil
	}
	var modules []GoModule
	for _, file := range snapshot.Files {
		if strings.ToLower(filepath.Base(file.Path)) != "go.mod" {
			continue
		}
		data, _, err := snapshot.Read(file.Path, 64*1024)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			fields := strings.Fields(line)
			if len(fields) == 2 && fields[0] == "module" && strings.TrimSpace(fields[1]) != "" {
				modules = append(modules, GoModule{Path: file.Path, Module: fields[1], Workspace: file.Workspace})
				break
			}
		}
	}
	sort.Slice(modules, func(i, j int) bool { return modules[i].Path < modules[j].Path })
	return modules
}

func qualityGates(snapshot *repository.RepositorySnapshot, analysis *Analysis) []QualityGate {
	if snapshot == nil || analysis == nil {
		return nil
	}
	paths := make(map[string]bool, len(snapshot.Files))
	for _, file := range snapshot.Files {
		paths[strings.ToLower(file.Path)] = true
	}
	has := func(name string) bool {
		name = strings.ToLower(name)
		if strings.HasPrefix(name, "*.") {
			extension := strings.TrimPrefix(name, "*")
			for path := range paths {
				if strings.HasSuffix(path, extension) {
					return true
				}
			}
			return false
		}
		if paths[name] {
			return true
		}
		for path := range paths {
			if filepath.Base(path) == name {
				return true
			}
		}
		return false
	}
	var gates []QualityGate
	add := func(id, command, reason, manifest string) {
		gate := QualityGate{ID: id, Command: command, Reason: reason}
		if workspace := manifestWorkspace(manifest); workspace != "" {
			gate.Workspace = workspace
			gate.Workspaces = []string{workspace}
		}
		gates = append(gates, gate)
	}
	addManifests := func(id, command, reason string, manifests []string) {
		for _, manifest := range manifests {
			gateID := id
			if workspace := manifestWorkspace(manifest); workspace != "." {
				gateID += "-" + strings.ReplaceAll(workspace, "/", "-")
			}
			add(gateID, command, reason, manifest)
		}
	}
	if has("go.mod") || hasFinding(analysis.Languages, "Go") {
		modules := manifestPaths(snapshot, "go.mod")
		if len(modules) > 0 {
			addManifests("go-test", "go test ./...", "Go module detected", modules)
		} else {
			add("go-test", "go test ./...", "Go source detected", firstPath(snapshot, "", ".go"))
		}
	}
	if has("package.json") {
		addManifests("npm-test", "npm test", "package.json detected; verify the repository test script before approval", manifestPaths(snapshot, "package.json"))
	}
	if has("pom.xml") {
		addManifests("maven-test", "mvn test", "Maven manifest detected", manifestPaths(snapshot, "pom.xml"))
	}
	if has("build.gradle") || has("build.gradle.kts") {
		command := "./gradlew test"
		if !has("gradlew") {
			command = "gradle test"
		}
		addManifests("gradle-test", command, "Gradle build detected", manifestPaths(snapshot, "build.gradle", "build.gradle.kts"))
	}
	if has("pyproject.toml") || has("pytest.ini") || has("tox.ini") {
		addManifests("python-test", "pytest", "Python test configuration detected", append(append(manifestPaths(snapshot, "pyproject.toml"), manifestPaths(snapshot, "pytest.ini")...), manifestPaths(snapshot, "tox.ini")...))
	}
	if has("cargo.toml") {
		addManifests("rust-test", "cargo test", "Cargo manifest detected", manifestPaths(snapshot, "cargo.toml"))
	}
	if has("*.sln") || has("*.csproj") {
		addManifests("dotnet-test", "dotnet test", ".NET project detected", manifestPathsBySuffix(snapshot, ".sln", ".csproj"))
	}
	return gates
}

func firstPath(snapshot *repository.RepositorySnapshot, exact, suffix string) string {
	if snapshot == nil {
		return ""
	}
	for _, file := range snapshot.Files {
		base := strings.ToLower(filepath.Base(file.Path))
		if base == strings.ToLower(exact) || (suffix != "" && strings.HasSuffix(base, strings.ToLower(suffix))) {
			return file.Path
		}
	}
	return ""
}

func manifestPaths(snapshot *repository.RepositorySnapshot, names ...string) []string {
	return manifestPathsBySuffix(snapshot, names...)
}

func manifestPathsBySuffix(snapshot *repository.RepositorySnapshot, names ...string) []string {
	if snapshot == nil {
		return nil
	}
	var paths []string
	for _, file := range snapshot.Files {
		base := strings.ToLower(filepath.Base(file.Path))
		for _, name := range names {
			name = strings.ToLower(name)
			if base == name || strings.HasSuffix(base, name) {
				paths = append(paths, file.Path)
				break
			}
		}
	}
	sort.Strings(paths)
	return paths
}

func manifestWorkspace(path string) string {
	if path == "" {
		return ""
	}
	directory := filepath.ToSlash(filepath.Dir(path))
	if directory == "." || directory == "" {
		return "."
	}
	return directory
}

func hasFinding(findings []Finding, value string) bool {
	for _, finding := range findings {
		if finding.Value == value {
			return true
		}
	}
	return false
}

func annotateWorkspaces(analysis *Analysis, snapshot *repository.RepositorySnapshot) {
	if analysis == nil || snapshot == nil {
		return
	}
	annotate := func(findings []Finding) []Finding {
		for i := range findings {
			workspaces := map[string]bool{}
			for _, evidence := range findings[i].Evidence {
				path := evidence
				quote := ""
				if marker := strings.Index(path, " contains "); marker >= 0 {
					quote = strings.TrimSpace(path[marker+len(" contains "):])
					path = path[:marker]
				}
				if workspace := snapshot.WorkspaceForPath(path); workspace != "" {
					workspaces[workspace] = true
				}
				if item := evidenceItem(snapshot, path, quote); item.Path != "" {
					findings[i].EvidenceItems = append(findings[i].EvidenceItems, item)
				}
			}
			if len(workspaces) > 0 {
				findings[i].Workspaces = make([]string, 0, len(workspaces))
				for workspace := range workspaces {
					findings[i].Workspaces = append(findings[i].Workspaces, workspace)
				}
				sort.Strings(findings[i].Workspaces)
				findings[i].Workspace = findings[i].Workspaces[0]
			}
		}
		return findings
	}
	analysis.Languages = annotate(analysis.Languages)
	analysis.Build = annotate(analysis.Build)
	analysis.Frameworks = annotate(analysis.Frameworks)
	analysis.Infrastructure = annotate(analysis.Infrastructure)
	analysis.Database = annotate(analysis.Database)
	analysis.Tests = annotate(analysis.Tests)
	analysis.Git = annotate(analysis.Git)
}

func evidenceItem(snapshot *repository.RepositorySnapshot, path, quote string) EvidenceItem {
	for _, file := range snapshot.Files {
		if filepath.ToSlash(filepath.Clean(file.Path)) != filepath.ToSlash(filepath.Clean(path)) {
			continue
		}
		item := EvidenceItem{Path: file.Path, Kind: file.Kind, Workspace: file.Workspace, Quote: quote}
		if data, _, err := snapshot.Read(file.Path, 1<<20); err == nil {
			hash := sha256.Sum256(data)
			item.SHA256 = fmt.Sprintf("%x", hash[:])
			item.StartLine, item.EndLine = evidenceLines(string(data), quote)
		}
		return item
	}
	return EvidenceItem{}
}

func evidenceLines(content, quote string) (int, int) {
	if strings.TrimSpace(quote) == "" {
		return 1, 1
	}
	line := 1
	for _, candidate := range strings.Split(content, "\n") {
		if strings.Contains(candidate, quote) {
			return line, line
		}
		line++
	}
	return 0, 0
}

type detectionResults struct {
	err            error
	languages      []Finding
	build          []Finding
	frameworks     []Finding
	infrastructure []Finding
	database       []Finding
	tests          []Finding
	git            []Finding
}

func runDetectors(ctx context.Context, repository Repository, options Options) detectionResults {
	var results detectionResults
	var mutex sync.Mutex
	var waitGroup sync.WaitGroup

	run := func(assign func([]Finding), detector Detector) {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			findings := detector.Detect(ctx, repository)

			mutex.Lock()
			defer mutex.Unlock()
			assign(findings)
		}()
	}

	run(func(findings []Finding) { results.languages = findings }, languageDetector{})
	run(func(findings []Finding) { results.build = findings }, buildDetector{})
	run(func(findings []Finding) { results.frameworks = findings }, frameworkDetector{})
	run(func(findings []Finding) { results.infrastructure = findings }, infrastructureDetector{})
	run(func(findings []Finding) { results.database = findings }, databaseDetector{})
	run(func(findings []Finding) { results.tests = findings }, testDetector{})

	if options.IncludeGit {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			findings, err := (gitDetector{}).Detect(ctx, repository)
			mutex.Lock()
			defer mutex.Unlock()
			results.git = findings
			results.err = err
		}()
	}

	waitGroup.Wait()
	return results
}

func collectFiles(repositoryPath string) ([]string, error) {
	return collectFilesContext(context.Background(), repositoryPath)
}
func collectFilesContext(ctx context.Context, repositoryPath string) ([]string, error) {
	snapshot, err := repository.Scan(ctx, repositoryPath, repository.ScanOptions{HonorIgnores: true})
	if err != nil {
		return nil, err
	}
	defer snapshot.Close()
	files := make([]string, 0, len(snapshot.Files))
	for _, file := range snapshot.Files {
		files = append(files, file.Path)
	}
	return files, nil
}

func finding(value string, evidence ...string) Finding {
	return findingWithStrength(value, "medium", evidence...)
}

func findingWithStrength(value, strength string, evidence ...string) Finding {
	return Finding{
		Value:         value,
		Confidence:    0,
		Strength:      strength,
		Evidence:      evidenceSample(evidence),
		EvidenceCount: len(evidence),
	}
}

func evidenceSample(evidence []string) []string {
	const limit = 10

	if len(evidence) <= limit {
		return evidence
	}

	return evidence[:limit]
}
