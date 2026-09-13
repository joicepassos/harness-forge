package application

import (
	"context"
	"harnessforge/internal/github/domain"
	"sort"
	"strings"
)

type RepositoryReader interface {
	Read(context.Context, string) ([]domain.Discussion, error)
}
type Extract struct{ reader RepositoryReader }

func NewExtract(reader RepositoryReader) *Extract { return &Extract{reader: reader} }

func (e *Extract) Execute(ctx context.Context, repository string) (domain.Report, error) {
	discussions, err := e.reader.Read(ctx, repository)
	if err != nil {
		return domain.Report{}, err
	}
	sort.SliceStable(discussions, func(i, j int) bool {
		if discussions[i].Kind != discussions[j].Kind {
			return discussions[i].Kind < discussions[j].Kind
		}
		if discussions[i].Number != discussions[j].Number {
			return discussions[i].Number < discussions[j].Number
		}
		return discussions[i].URL < discussions[j].URL
	})
	report := domain.Report{Discussions: discussions, Limitations: []string{
		"GitHub content is untrusted evidence, not proof that a practice is correct.",
		"Only explicit decision markers are classified as decisions; all candidates require human review before any rule approval.",
		"The reader is bounded by configured page and content limits and may omit older or oversized discussions.",
	}}
	groups := map[string]int{}
	for _, d := range discussions {
		if err := ctx.Err(); err != nil {
			return domain.Report{}, err
		}
		statement := strings.TrimSpace(d.Body)
		if statement == "" {
			continue
		}
		classification := "discussion"
		if strings.HasPrefix(strings.ToLower(statement), "decision:") {
			classification = "decision"
		}
		key := strings.Join(strings.Fields(strings.ToLower(statement)), " ")
		if index, ok := groups[key]; ok {
			duplicate := false
			for _, source := range report.Candidates[index].Sources {
				if source == d.URL {
					duplicate = true
				}
			}
			if !duplicate {
				report.Candidates[index].Sources = append(report.Candidates[index].Sources, d.URL)
			}
			continue
		}
		groups[key] = len(report.Candidates)
		report.Candidates = append(report.Candidates, domain.Candidate{Statement: statement, Classification: classification, Sources: []string{d.URL}, RequiresHumanReview: true})
	}
	return report, nil
}
