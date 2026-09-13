package infrastructure

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"harnessforge/internal/plugins/domain"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const maxPluginMessage = 1024 * 1024

var pluginName = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)

type Local struct{}

func (Local) Discover(ctx context.Context, directory string) ([]domain.Manifest, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	root, err := filepath.Abs(directory)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("discover plugins: %w", err)
	}
	var manifests []domain.Manifest
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("plugin manifest %q must not be a symbolic link", entry.Name())
		}
		if info.Size() > 64*1024 {
			return nil, fmt.Errorf("plugin manifest %q exceeds 64 KiB", entry.Name())
		}
		data, err := os.ReadFile(filepath.Join(root, entry.Name()))
		if err != nil {
			return nil, err
		}
		var manifest domain.Manifest
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&manifest); err != nil {
			return nil, fmt.Errorf("invalid plugin manifest %q: %w", entry.Name(), err)
		}
		var trailing any
		if err := decoder.Decode(&trailing); err != io.EOF {
			return nil, fmt.Errorf("invalid plugin manifest %q: trailing content", entry.Name())
		}
		if manifest.APIVersion == "" || !pluginName.MatchString(manifest.Name) || len(manifest.Command) == 0 || len(manifest.Capabilities) == 0 {
			return nil, fmt.Errorf("invalid plugin manifest %q: api_version, safe name, command and capabilities are required", entry.Name())
		}
		manifests = append(manifests, manifest)
	}
	sort.Slice(manifests, func(i, j int) bool { return manifests[i].Name < manifests[j].Name })
	for i := 1; i < len(manifests); i++ {
		if manifests[i-1].Name == manifests[i].Name {
			return nil, fmt.Errorf("duplicate plugin name %q", manifests[i].Name)
		}
	}
	return manifests, nil
}

func (Local) Execute(ctx context.Context, directory string, manifest domain.Manifest, request domain.Request) (domain.Response, error) {
	payload, err := json.Marshal(request)
	if err != nil {
		return domain.Response{}, err
	}
	if len(payload) > maxPluginMessage {
		return domain.Response{}, fmt.Errorf("plugin request exceeds 1 MiB")
	}
	command := exec.CommandContext(ctx, manifest.Command[0], manifest.Command[1:]...)
	command.Dir = directory
	command.Env = safeEnvironment()
	command.Stdin = bytes.NewReader(payload)
	var stdout, stderr limitedBuffer
	stdout.remaining, stderr.remaining = maxPluginMessage, maxPluginMessage
	command.Stdout, command.Stderr = &stdout, &stderr
	if err := command.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message != "" {
			return domain.Response{}, fmt.Errorf("%w: %s", err, message)
		}
		return domain.Response{}, err
	}
	if stdout.exceeded {
		return domain.Response{}, fmt.Errorf("plugin response exceeds 1 MiB")
	}
	var response domain.Response
	decoder := json.NewDecoder(bytes.NewReader(stdout.Bytes()))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&response); err != nil {
		return response, fmt.Errorf("invalid plugin response: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return response, fmt.Errorf("invalid plugin response: trailing content")
	}
	return response, nil
}

type limitedBuffer struct {
	bytes.Buffer
	remaining int
	exceeded  bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	original := len(p)
	if len(p) > b.remaining {
		p = p[:b.remaining]
		b.exceeded = true
	}
	_, _ = b.Buffer.Write(p)
	b.remaining -= len(p)
	return original, nil
}

func safeEnvironment() []string {
	keys := []string{"PATH", "Path", "SystemRoot", "SYSTEMROOT", "TEMP", "TMP", "TMPDIR", "HOME", "LOCALAPPDATA", "GOCACHE", "GOPATH"}
	env := make([]string, 0, len(keys))
	for _, key := range keys {
		if value := os.Getenv(key); value != "" {
			env = append(env, key+"="+value)
		}
	}
	return env
}
