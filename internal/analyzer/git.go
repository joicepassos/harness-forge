package analyzer

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type gitDetector struct{}

func (gitDetector) Detect(ctx context.Context, repository Repository) ([]Finding, error) {
	commands := []gitCommand{
		{"Repository", []string{"rev-parse", "--show-toplevel"}},
		{"Branch", []string{"branch", "--show-current"}},
		{"Branches", []string{"branch", "--all", "--format=%(refname:short)"}},
		{"Changed files (working tree)", []string{"-c", "core.quotePath=false", "status", "--short"}},
	}
	findings, err := applyGitCommands(ctx, repository, commands)
	if err != nil {
		return nil, err
	}
	// An initialized repository with no commits is valid; symbolic HEAD names an unborn branch.
	_, headErr := runGit(ctx, repository, "rev-parse", "--verify", "HEAD")
	if headErr != nil {
		branch, err := runGit(ctx, repository, "symbolic-ref", "--short", "HEAD")
		if err != nil {
			return nil, headErr
		}
		_, err = runGit(ctx, repository, "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
		if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 1 {
			return nil, headErr
		}
		return append(findings, finding("Commits: 0")), nil
	}
	history, err := applyGitCommands(ctx, repository, []gitCommand{
		{"Last commit", []string{"log", "-1", "--pretty=%h %s"}},
		{"Commits", []string{"rev-list", "--count", "HEAD"}},
		{"Contributors", []string{"shortlog", "-sn", "HEAD"}},
		{"Recent commits (max 20)", []string{"log", "-20", "--pretty=%h %s"}},
		{"Files changed in recent commits (max 20)", []string{"-c", "core.quotePath=false", "log", "-20", "--pretty=format:", "--name-only"}},
	})
	return append(findings, history...), err
}

type gitCommand struct {
	value string
	args  []string
}

func applyGitCommands(ctx context.Context, repository Repository, commands []gitCommand) ([]Finding, error) {
	findings := make([]Finding, 0, len(commands))
	for _, command := range commands {
		output, err := runGit(ctx, repository, command.args...)
		if err != nil {
			return nil, fmt.Errorf("git %s: %w", command.value, err)
		}
		if output == "" {
			output = "(none)"
		}
		findings = append(findings, finding(command.value+": "+singleLine(output), strings.Split(output, "\n")...))
	}
	return findings, nil
}
func runGit(ctx context.Context, repository Repository, args ...string) (string, error) {
	command := exec.CommandContext(ctx, "git", args...)
	command.Dir = repository.Path
	output, err := command.Output()
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	return strings.TrimSpace(string(output)), err
}
func singleLine(text string) string {
	lines := strings.FieldsFunc(text, func(c rune) bool { return c == '\n' || c == '\r' })
	for i, line := range lines {
		lines[i] = strings.Join(strings.Fields(line), " ")
	}
	return strings.Join(lines, "; ")
}
