package analyzer

import (
	"os"
	"path/filepath"
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

type Repository struct {
	Path  string
	Files []string
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

	repository := Repository{
		Path:  absolutePath,
		Files: files,
	}

	analysis.Files = len(files)
	analysis.Languages = languageDetector{}.Detect(repository)
	analysis.Build = buildDetector{}.Detect(repository)
	analysis.Frameworks = frameworkDetector{}.Detect(repository)
	analysis.Infrastructure = infrastructureDetector{}.Detect(repository)
	analysis.Database = databaseDetector{}.Detect(repository)
	analysis.Tests = testDetector{}.Detect(repository)

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
			case ".git", ".harness", ".next", "build", "dist", "node_modules", "target":
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

func finding(value string, evidence ...string) Finding {
	return Finding{
		Value:      value,
		Confidence: 1.0,
		Evidence:   evidence,
	}
}
