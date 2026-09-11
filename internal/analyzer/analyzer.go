package analyzer

import (
	"os"
	"path/filepath"
	"strings"
)

type Analysis struct {
	Project        string
	Languages      []Finding
	Build          []Finding
	Frameworks     []Finding
	Infrastructure []Finding
	Database       []Finding
	Tests          []Finding
	Files          int
}

type Finding struct {
	Value      string
	Confidence float64
	Evidence   []string
}

func Analyze(repositoryPath string) (*Analysis, error) {
	absolutePath, err := filepath.Abs(repositoryPath)
	if err != nil {
		return nil, err
	}

	analysis := &Analysis{
		Project: filepath.Base(absolutePath),
	}

	files, err := collectFiles(absolutePath)
	if err != nil {
		return nil, err
	}

	analysis.Files = len(files)
	analysis.Languages = detectLanguages(files)
	analysis.Build = detectBuild(files)
	analysis.Frameworks = detectFrameworks(absolutePath)
	analysis.Infrastructure = detectInfrastructure(files)
	analysis.Database = detectDatabase(absolutePath, files)
	analysis.Tests = detectTests(files)

	return analysis, nil
}

func collectFiles(repositoryPath string) ([]string, error) {
	var files []string

	err := filepath.WalkDir(repositoryPath, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if entry.IsDir() {
			switch entry.Name() {
			case ".git", ".harness":
				return filepath.SkipDir
			}
			return nil
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

func hasFile(files []string, name string) bool {
	for _, file := range files {
		if strings.EqualFold(filepath.Base(file), name) || strings.EqualFold(file, name) {
			return true
		}
	}

	return false
}

func hasPrefix(files []string, prefix string) bool {
	for _, file := range files {
		if strings.HasPrefix(file, prefix) {
			return true
		}
	}

	return false
}

func filesWithExtension(files []string, extension string) []string {
	var matches []string

	for _, file := range files {
		if strings.EqualFold(filepath.Ext(file), extension) {
			matches = append(matches, file)
		}
	}

	return matches
}

func finding(value string, evidence ...string) Finding {
	return Finding{
		Value:      value,
		Confidence: 1.0,
		Evidence:   evidence,
	}
}
