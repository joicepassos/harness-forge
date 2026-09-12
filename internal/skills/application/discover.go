package application

import (
	"context"
	"fmt"
	"harnessforge/internal/skills/domain"
	"path/filepath"
	"sort"
	"strings"
)

type FileReader interface {
	Files(context.Context, string) ([]File, error)
}

type File struct {
	Path string
	Text string
}

type Discover struct{ reader FileReader }

func NewDiscover(reader FileReader) *Discover { return &Discover{reader: reader} }

func (d *Discover) Execute(ctx context.Context, repository string) ([]domain.Proposal, error) {
	files, err := d.reader.Files(ctx, repository)
	if err != nil {
		return nil, err
	}
	roles := map[string][]domain.Evidence{}
	for _, file := range files {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		role, symbol := classify(file)
		if role != "" {
			roles[role] = append(roles[role], domain.Evidence{File: file.Path, Symbol: symbol})
		}
	}
	if len(roles["controller"]) < 2 || len(roles["service"]) < 2 {
		return nil, nil
	}
	required := []string{"controller", "service", "persistence", "migration", "integration-test"}
	var examples []domain.Evidence
	for _, role := range required {
		if len(roles[role]) > 0 {
			examples = append(examples, roles[role][0])
		}
	}
	if len(examples) < 4 {
		return nil, nil
	}
	sort.Slice(examples, func(i, j int) bool { return examples[i].File < examples[j].File })
	return []domain.Proposal{{ID: "add-backed-feature", Description: "Add a backend feature through a controller, service, persistence or migration, and integration test.", Examples: examples, Limitations: []string{"This is a heuristic proposal based on file and symbol evidence.", "Review project conventions and security requirements before using it."}}}, nil
}

func classify(file File) (string, string) {
	path := filepath.ToSlash(file.Path)
	text := file.Text
	switch {
	case strings.HasSuffix(path, ".sql") && (strings.Contains(strings.ToUpper(text), "CREATE TABLE") || strings.Contains(strings.ToUpper(text), "ALTER TABLE")):
		return "migration", "migration"
	case strings.HasSuffix(path, "Test.java") && (strings.Contains(text, "IntegrationTest") || strings.Contains(text, "@SpringBootTest")):
		return "integration-test", "integration test"
	case strings.Contains(text, "@RestController") || strings.Contains(text, "@RequestMapping"):
		return "controller", "controller"
	case strings.Contains(text, "@Service") || strings.Contains(text, "implements ") && strings.Contains(path, "service"):
		return "service", "service"
	case strings.Contains(text, "@Repository") || strings.Contains(text, "JdbcClient") || strings.Contains(text, "Repository"):
		return "persistence", "persistence"
	}
	return "", ""
}

func ValidateID(id string) error {
	if id == "" || strings.Contains(id, "/") || strings.Contains(id, "\\") || strings.Contains(id, "..") {
		return fmt.Errorf("invalid skill id")
	}
	for _, r := range id {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-') {
			return fmt.Errorf("invalid skill id")
		}
	}
	return nil
}

func ValidateEvidence(evidence []domain.Evidence) error {
	for _, item := range evidence {
		path := strings.ReplaceAll(item.File, "\\", "/")
		if path == "" || strings.HasPrefix(path, "/") || strings.Contains(path, "../") || path == ".." {
			return fmt.Errorf("invalid skill evidence path")
		}
	}
	return nil
}
