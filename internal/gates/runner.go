// Package gates provides the explicitly enabled project quality-gate runner.
package gates

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const DefaultTimeout = 5 * time.Minute
const outputLimit = 16 << 10

type Gate struct {
	ID         string
	Command    string
	Workspace  string
	Workspaces []string
	Env        map[string]string
}
type Result struct {
	ID              string `json:"id"`
	Command         string `json:"command"`
	Workspace       string `json:"workspace"`
	Status          string `json:"status"`
	ExitCode        int    `json:"exit_code"`
	DurationMS      int64  `json:"duration_ms"`
	TimedOut        bool   `json:"timed_out"`
	Output          string `json:"output,omitempty"`
	OutputTruncated bool   `json:"output_truncated,omitempty"`
	Error           string `json:"error,omitempty"`
}

func Run(ctx context.Context, root string, items []Gate, timeout time.Duration) ([]Result, error) {
	if timeout <= 0 {
		return nil, fmt.Errorf("gate timeout must be positive")
	}
	for _, gate := range items {
		if err := validateEnv(gate.Env); err != nil {
			return nil, fmt.Errorf("quality gate %q environment: %w", gate.ID, err)
		}
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	info, err := os.Lstat(abs)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("repository must be a regular directory")
	}
	results := []Result{}
	for _, gate := range items {
		workspaces := gate.Workspaces
		if len(workspaces) == 0 {
			if gate.Workspace != "" {
				workspaces = []string{gate.Workspace}
			} else {
				workspaces = []string{"."}
			}
		}
		for _, workspace := range workspaces {
			if err := ctx.Err(); err != nil {
				return results, err
			}
			cwd, err := resolveWorkspace(abs, workspace)
			result := Result{ID: gate.ID, Command: gate.Command, Workspace: filepath.ToSlash(workspace), Status: "failed", ExitCode: -1}
			if err != nil {
				result.Error = err.Error()
				results = append(results, result)
				continue
			}
			runCtx, cancel := context.WithTimeout(ctx, timeout)
			started := time.Now()
			var command *exec.Cmd
			if runtime.GOOS == "windows" {
				command = exec.CommandContext(runCtx, "cmd.exe", "/C", gate.Command)
			} else {
				command = exec.CommandContext(runCtx, "/bin/sh", "-c", gate.Command)
			}
			configureProcessTree(command)
			command.Dir = cwd
			command.Env = mergeEnv(os.Environ(), gate.Env)
			capture := &limitedOutput{}
			command.Stdout = capture
			command.Stderr = capture
			err = command.Run()
			gateContextErr := runCtx.Err()
			parentContextErr := ctx.Err()
			cancel()
			result.DurationMS = time.Since(started).Milliseconds()
			result.Output = capture.String()
			result.OutputTruncated = capture.truncated
			if err == nil {
				result.Status = "passed"
				result.ExitCode = 0
			} else {
				result.Error = err.Error()
				if gateContextErr == context.DeadlineExceeded {
					result.TimedOut = true
					result.Error = "quality gate timed out"
				} else if parentContextErr != nil {
					return results, parentContextErr
				} else {
					var exit *exec.ExitError
					if errors.As(err, &exit) {
						result.ExitCode = exit.ExitCode()
					}
				}
			}
			results = append(results, result)
		}
	}
	return results, nil
}

func validateEnv(env map[string]string) error {
	for key, value := range env {
		if key == "" || !envStart(key[0]) {
			return fmt.Errorf("invalid environment key %q", key)
		}
		for i := 1; i < len(key); i++ {
			if !envPart(key[i]) {
				return fmt.Errorf("invalid environment key %q", key)
			}
		}
		if strings.IndexByte(value, 0) >= 0 {
			return fmt.Errorf("environment value for %q contains NUL", key)
		}
	}
	return nil
}
func envStart(c byte) bool { return c == '_' || c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' }
func envPart(c byte) bool  { return envStart(c) || c >= '0' && c <= '9' }
func mergeEnv(parent []string, overrides map[string]string) []string {
	key := func(s string) string {
		if runtime.GOOS == "windows" {
			return strings.ToUpper(s)
		}
		return s
	}
	values := make(map[string]string, len(parent)+len(overrides))
	order := make([]string, 0, len(parent)+len(overrides))
	for _, entry := range parent {
		sep := strings.IndexByte(entry, '=')
		if sep <= 0 {
			continue
		}
		k := entry[:sep]
		n := key(k)
		if _, ok := values[n]; !ok {
			order = append(order, n)
		}
		values[n] = entry
	}
	for k, v := range overrides {
		n := key(k)
		if _, ok := values[n]; !ok {
			order = append(order, n)
		}
		values[n] = k + "=" + v
	}
	result := make([]string, 0, len(order))
	for _, n := range order {
		result = append(result, values[n])
	}
	return result
}

func resolveWorkspace(root, workspace string) (string, error) {
	if strings.TrimSpace(workspace) == "" || filepath.IsAbs(workspace) || strings.Contains(workspace, "\\") || strings.Contains(workspace, ":") {
		return "", fmt.Errorf("workspace must be repository-relative")
	}
	clean := filepath.Clean(filepath.FromSlash(workspace))
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("workspace escapes repository")
	}
	target := filepath.Join(root, clean)
	relative, err := filepath.Rel(root, target)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("workspace escapes repository")
	}
	current := root
	for _, part := range strings.Split(relative, string(filepath.Separator)) {
		if part == "." || part == "" {
			continue
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			return "", err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("workspace contains a symlink")
		}
		if !info.IsDir() {
			return "", fmt.Errorf("workspace is not a directory")
		}
	}
	return target, nil
}

type limitedOutput struct {
	bytes.Buffer
	truncated bool
}

func (w *limitedOutput) Write(p []byte) (int, error) {
	remaining := outputLimit - w.Len()
	if remaining > 0 {
		n := min(remaining, len(p))
		_, _ = w.Buffer.Write(p[:n])
	}
	if len(p) > max(remaining, 0) {
		w.truncated = true
	}
	return len(p), nil
}
