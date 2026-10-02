package main

import (
	"fmt"
	"harnessforge/internal/analyzer"
	"strings"
)

func (s setupSession) adjustDetectedLanguages(analysis *analyzer.Analysis) ([]string, error) {
	var detected []string
	for _, finding := range analysis.Languages {
		detected = append(detected, finding.Value)
	}
	if s.interactive {
		selected, err := s.selectLanguages([]string{"Go", "JavaScript/TypeScript", "Python", "Java", "Rust", "C/C++", "Other"}, detected)
		if err != nil {
			return nil, err
		}
		findings := make([]analyzer.Finding, 0, len(selected))
		for _, value := range selected {
			finding := analyzer.Finding{Value: value}
			for _, existing := range analysis.Languages {
				if existing.Value == value {
					finding = existing
					break
				}
			}
			findings = append(findings, finding)
		}
		analysis.Languages = findings
		return selected, nil
	}
	if len(detected) > 0 {
		fmt.Fprintf(s.output, "Detected languages: %s\n", strings.Join(detected, ", "))
		for {
			answer, err := s.ask("Languages [Enter: use detected; 2: adjust or add]: ")
			if err != nil {
				return nil, err
			}
			if answer == "" || answer == "1" {
				return detected, nil
			}
			if answer == "2" {
				break
			}
			fmt.Fprintln(s.output, "Choose Enter to use detected languages, or 2 to adjust.")
		}
	} else {
		fmt.Fprintln(s.output, "No programming languages detected; choose languages to guide the harness.")
	}
	selected, err := s.chooseLanguages()
	if err != nil {
		return nil, err
	}
	if len(selected) == 1 && selected[0] == "all detected" {
		return detected, nil
	}
	findings := make([]analyzer.Finding, 0, len(selected))
	for _, value := range selected {
		finding := analyzer.Finding{Value: value}
		for _, existing := range analysis.Languages {
			if existing.Value == value {
				finding = existing
				break
			}
		}
		findings = append(findings, finding)
	}
	analysis.Languages = findings
	return selected, nil
}
