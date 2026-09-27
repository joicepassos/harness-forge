package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestServeInitializeListAndRead(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Sample\nA small sample repository.\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".forge"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".forge", "forge.yaml"), []byte("layout_version: 1\nir_version: 2\nproject: {name: sample}\ntargets: [codex]\nreferences: {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"resources/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"resources/read","params":{"uri":"forge://context/current"}}`,
	}, "\n")
	var out strings.Builder
	err := (Server{Repository: root, Budget: 512}).Serve(context.Background(), strings.NewReader(input), &out)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected three responses, got %d: %s", len(lines), out.String())
	}
	var initialize map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &initialize); err != nil {
		t.Fatal(err)
	}
	if initialize["error"] != nil {
		t.Fatalf("initialize failed: %s", lines[0])
	}
	result := initialize["result"].(map[string]any)
	if result["protocolVersion"] != ProtocolVersion {
		t.Fatalf("protocol version = %v", result["protocolVersion"])
	}
	var listed map[string]any
	if err := json.Unmarshal([]byte(lines[1]), &listed); err != nil {
		t.Fatal(err)
	}
	if listed["error"] != nil {
		t.Fatalf("list failed: %s", lines[1])
	}
	resources := listed["result"].(map[string]any)["resources"].([]any)
	if len(resources) != 1 || resources[0].(map[string]any)["uri"] != contextResourceURI {
		t.Fatalf("unexpected resources: %#v", resources)
	}
	var read map[string]any
	if err := json.Unmarshal([]byte(lines[2]), &read); err != nil {
		t.Fatal(err)
	}
	if read["error"] != nil {
		t.Fatalf("read failed: %s", lines[2])
	}
	contents := read["result"].(map[string]any)["contents"].([]any)
	if len(contents) != 1 || contents[0].(map[string]any)["mimeType"] != "application/json" {
		t.Fatalf("unexpected contents: %#v", contents)
	}
	var plan map[string]any
	if err := json.Unmarshal([]byte(contents[0].(map[string]any)["text"].(string)), &plan); err != nil {
		t.Fatalf("resource is not JSON: %v", err)
	}
}

func TestServeErrorsAndLimits(t *testing.T) {
	t.Run("requires initialize", func(t *testing.T) {
		var out strings.Builder
		err := (Server{Repository: t.TempDir(), Budget: 1}).Serve(context.Background(), strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"resources/list"}`), &out)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), `"code":-32002`) {
			t.Fatalf("expected not initialized error: %s", out.String())
		}
	})
	t.Run("unknown resource", func(t *testing.T) {
		input := "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"initialize\",\"params\":{\"protocolVersion\":\"x\"}}\n" +
			"{\"jsonrpc\":\"2.0\",\"method\":\"notifications/initialized\"}\n" +
			"{\"jsonrpc\":\"2.0\",\"id\":2,\"method\":\"resources/read\",\"params\":{\"uri\":\"forge://other\"}}\n"
		var out strings.Builder
		if err := (Server{Repository: t.TempDir(), Budget: 1}).Serve(context.Background(), strings.NewReader(input), &out); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), `"code":-32602`) {
			t.Fatalf("expected invalid params: %s", out.String())
		}
	})
	t.Run("oversized input", func(t *testing.T) {
		input := strings.NewReader(strings.Repeat("x", MaxMessageBytes+1) + "\n")
		var out strings.Builder
		err := (Server{Repository: t.TempDir(), Budget: 1}).Serve(context.Background(), input, &out)
		if err == nil || !strings.Contains(err.Error(), "byte limit") {
			t.Fatalf("expected bounded message error, got %v", err)
		}
	})
	t.Run("configuration limits", func(t *testing.T) {
		var out strings.Builder
		if err := (Server{Repository: t.TempDir()}).Serve(context.Background(), strings.NewReader(""), &out); err == nil {
			t.Fatal("expected positive budget requirement")
		}
	})
}

func TestServeRejectsInvalidJSON(t *testing.T) {
	var out strings.Builder
	if err := (Server{Repository: t.TempDir(), Budget: 1}).Serve(context.Background(), strings.NewReader("{bad}\n"), &out); err != nil {
		t.Fatal(err)
	}
	var response rpcResponse
	if err := json.Unmarshal([]byte(strings.TrimSpace(out.String())), &response); err != nil {
		t.Fatal(err)
	}
	if response.Error == nil || response.Error.Code != -32700 {
		t.Fatalf("expected parse error: %s", fmt.Sprint(out.String()))
	}
}
