package analyzer

import (
	"context"
	"os/exec"
	"strings"
)

type gitDetector struct{}

func (gitDetector) Detect(ctx context.Context, repository Repository) []Finding {
	return applyGitCommands(ctx, repository, []gitCommand{
		{value: "Repository", args: []string{"rev-parse", "--show-toplevel"}},
		{value: "Branch", args: []string{"branch", "--show-current"}},
		{value: "Last commit", args: []string{"log", "-1", "--pretty=%h %s"}},
		{value: "Commits", args: []string{"rev-list", "--count", "HEAD"}},
		{value: "Contributors", args: []string{"shortlog", "-sn", "HEAD"}},
	})
}

type gitCommand struct {
	value string
	args  []string
}

func applyGitCommands(ctx context.Context, repository Repository, commands []gitCommand) []Finding {
	findings := make([]Finding, 0, len(commands))

	for _, command := range commands {
		output, ok := runGit(ctx, repository, command.args...)
		if ok {
			findings = append(findings, finding(command.value+": "+singleLine(output), output))
		}
	}

	return findings
}

func runGit(ctx context.Context, repository Repository, args ...string) (string, bool) {
	command := exec.CommandContext(ctx, "git", args...)
	command.Dir = repository.Path

	output, err := command.Output()
	if err != nil {
		return "", false
	}

	text := strings.TrimSpace(string(output))
	return text, text != ""
}

func singleLine(text string) string {
	lines := strings.FieldsFunc(text, func(character rune) bool {
		return character == '\n' || character == '\r'
	})

	cleanLines := make([]string, 0, len(lines))
	for _, line := range lines {
		cleanLines = append(cleanLines, strings.Join(strings.Fields(line), " "))
	}

	return strings.Join(cleanLines, "; ")
}
