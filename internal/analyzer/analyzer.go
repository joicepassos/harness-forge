package analyzer

import (
	"context"
	"fmt"
	"harnessforge/internal/inputlimits"
	"harnessforge/internal/securityboundary"
	"os"
	"path/filepath"
	"sync"
)

type Analysis struct {
	Project        string    `json:"project"`
	Languages      []Finding `json:"languages,omitempty"`
	Build          []Finding `json:"build,omitempty"`
	Frameworks     []Finding `json:"frameworks,omitempty"`
	Structure      []Finding `json:"structure,omitempty"`
	Architecture   []Finding `json:"architecture,omitempty"`
	Conventions    []Finding `json:"conventions,omitempty"`
	Infrastructure []Finding `json:"infrastructure,omitempty"`
	Database       []Finding `json:"database,omitempty"`
	Tests          []Finding `json:"tests,omitempty"`
	Git            []Finding `json:"git,omitempty"`
	Files          int       `json:"files"`
}

type Finding struct {
	Value         string   `json:"value"`
	Confidence    float64  `json:"confidence"`
	Evidence      []string `json:"evidence,omitempty"`
	EvidenceCount int      `json:"evidence_count,omitempty"`
}

type Repository struct {
	Path  string
	Files []string
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

	analysis := &Analysis{
		Project: filepath.Base(absolutePath),
	}

	files, err := collectFilesContext(ctx, absolutePath)
	if err != nil {
		return nil, err
	}

	repository := Repository{
		Path:  absolutePath,
		Files: files,
	}

	analysis.Files = len(files)
	analysis.Structure, analysis.Architecture = detectLayout(files)
	analysis.Conventions = detectConventions(files)
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

	return analysis, nil
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
	var files []string

	err := filepath.WalkDir(repositoryPath, func(path string, entry os.DirEntry, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			return err
		}

		if entry.IsDir() {
			if securityboundary.SkipRepositoryDirectory(entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 || !entry.Type().IsRegular() {
			return nil
		}
		if len(files) >= inputlimits.RepositoryFiles {
			return fmt.Errorf("repository scan exceeds %d files", inputlimits.RepositoryFiles)
		}

		relativePath, err := filepath.Rel(repositoryPath, path)
		if err != nil {
			return err
		}

		files = append(files, filepath.ToSlash(relativePath))
		return nil
	})
	if err != nil {
		return nil, err
	}

	return files, nil
}

func finding(value string, evidence ...string) Finding {
	return Finding{
		Value:         value,
		Confidence:    1.0,
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
