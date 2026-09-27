package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path/filepath"
	"strings"
)

// MaxMessageBytes bounds each newline-delimited JSON-RPC message accepted by Serve.
const MaxMessageBytes = 1 << 20

// MaxResourceBytes caps serialized resource bodies before JSON-RPC framing.
const MaxResourceBytes = MaxMessageBytes - 4096

const contextResourceURI = "forge://context/current"
const contextResourceTemplateURI = "forge://context/task/{prompt}{?path*}"

type Server struct {
	Resolver   Resolver
	Repository string
	Model      string
	Budget     int
}

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type rpcResponse struct {
	JSONRPC string    `json:"jsonrpc"`
	ID      any       `json:"id"`
	Result  any       `json:"result,omitempty"`
	Error   *rpcError `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Serve handles MCP JSON-RPC over a newline-delimited stream, suitable for stdio
// transports. It never invokes tools, commands, or writes into the repository.
func (s Server) Serve(ctx context.Context, input io.Reader, output io.Writer) error {
	if s.Repository == "" {
		return errors.New("MCP repository is required")
	}
	if s.Budget < 1 {
		return errors.New("MCP context budget must be positive")
	}
	r := bufio.NewReaderSize(input, 4096)
	w := bufio.NewWriter(output)
	initializeResponded := false
	initialized := false
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		line, err := readMessage(r)
		if errors.Is(err, io.EOF) {
			return w.Flush()
		}
		if err != nil {
			return err
		}
		if len(strings.TrimSpace(string(line))) == 0 {
			continue
		}
		var req rpcRequest
		if err := json.Unmarshal(line, &req); err != nil {
			if e := writeResponse(w, rpcResponse{JSONRPC: "2.0", ID: nil, Error: &rpcError{Code: -32700, Message: "parse error"}}); e != nil {
				return e
			}
			continue
		}
		if req.JSONRPC != "2.0" || req.Method == "" {
			if e := writeResponse(w, rpcResponse{JSONRPC: "2.0", ID: decodeID(req.ID), Error: &rpcError{Code: -32600, Message: "invalid request"}}); e != nil {
				return e
			}
			continue
		}
		if len(req.ID) == 0 { // notifications never receive a response
			if req.Method == "notifications/initialized" && initializeResponded {
				initialized = true
			}
			continue
		}
		id := decodeID(req.ID)
		var result any
		var rpcErr *rpcError
		switch req.Method {
		case "initialize":
			var p struct {
				ProtocolVersion string `json:"protocolVersion"`
				ClientInfo      any    `json:"clientInfo"`
				Capabilities    any    `json:"capabilities"`
			}
			if err := json.Unmarshal(req.Params, &p); err != nil || p.ProtocolVersion == "" {
				rpcErr = &rpcError{Code: -32602, Message: "initialize requires protocolVersion"}
			} else if initializeResponded {
				rpcErr = &rpcError{Code: -32600, Message: "server is already initialized"}
			} else {
				initializeResponded = true
				result = map[string]any{"protocolVersion": ProtocolVersion, "capabilities": map[string]any{"resources": map[string]any{"listChanged": false}}, "serverInfo": map[string]string{"name": "harnessforge", "version": "1"}, "instructions": "Read-only Forge context resource. Resource reads do not execute project commands."}
			}
		case "ping":
			result = map[string]any{}
		case "resources/list":
			if !initialized {
				rpcErr = notInitialized()
			} else {
				result = map[string]any{"resources": []any{map[string]any{"uri": contextResourceURI, "name": "Forge task context", "description": "Bounded, provenance-bearing context plan for the current repository.", "mimeType": "application/json"}}}
			}
		case "resources/templates/list":
			if !initialized {
				rpcErr = notInitialized()
			} else {
				result = map[string]any{"resourceTemplates": []any{map[string]any{"uriTemplate": contextResourceTemplateURI, "name": "Forge context for a task", "description": "Read a context plan for a URL-encoded task prompt; optional repeated path query parameters scope knowledge to repository-relative task files.", "mimeType": "application/json"}}}
			}
		case "resources/read":
			if !initialized {
				rpcErr = notInitialized()
				break
			}
			var p struct {
				URI string `json:"uri"`
			}
			if err := json.Unmarshal(req.Params, &p); err != nil || p.URI == "" {
				rpcErr = &rpcError{Code: -32602, Message: "resources/read requires uri"}
				break
			}
			prompt := "repository context"
			var taskPaths []string
			if p.URI != contextResourceURI {
				parsed, parseErr := url.Parse(p.URI)
				if parseErr != nil || parsed.Scheme != "forge" || parsed.Host != "context" || parsed.Fragment != "" || parsed.User != nil {
					rpcErr = &rpcError{Code: -32602, Message: "unknown resource URI"}
					break
				}
				if !strings.HasPrefix(parsed.EscapedPath(), "/task/") {
					rpcErr = &rpcError{Code: -32602, Message: "unknown resource URI"}
					break
				}
				prompt, parseErr = url.PathUnescape(strings.TrimPrefix(parsed.EscapedPath(), "/task/"))
				if parseErr != nil || strings.TrimSpace(prompt) == "" || len(prompt) > 64<<10 {
					rpcErr = &rpcError{Code: -32602, Message: "task prompt is invalid or too large"}
					break
				}
				query := parsed.Query()
				for key := range query {
					if key != "path" {
						rpcErr = &rpcError{Code: -32602, Message: "unsupported context resource query parameter"}
						break
					}
				}
				if rpcErr != nil {
					break
				}
				taskPaths = query["path"]
				for _, taskPath := range taskPaths {
					clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(taskPath)))
					if taskPath == "" || filepath.IsAbs(taskPath) || strings.Contains(taskPath, "\\") || strings.Contains(taskPath, ":") || clean == ".." || strings.HasPrefix(clean, "../") {
						rpcErr = &rpcError{Code: -32602, Message: "task paths must be repository-relative"}
						break
					}
				}
				if rpcErr != nil {
					break
				}
			}
			data, err := s.Resolver.ReadResource(ctx, s.Repository, prompt, s.Model, s.Budget, taskPaths...)
			if err != nil {
				rpcErr = &rpcError{Code: -32603, Message: "unable to resolve Forge context"}
				break
			}
			if len(data) > MaxResourceBytes {
				rpcErr = &rpcError{Code: -32603, Message: "resolved Forge context exceeds resource size limit"}
				break
			}
			result = map[string]any{"contents": []any{map[string]any{"uri": p.URI, "mimeType": "application/json", "text": string(data)}}}
		default:
			rpcErr = &rpcError{Code: -32601, Message: "method not found"}
		}
		resp := rpcResponse{JSONRPC: "2.0", ID: id, Result: result, Error: rpcErr}
		if err := writeResponse(w, resp); err != nil {
			return err
		}
	}
}

func notInitialized() *rpcError { return &rpcError{Code: -32002, Message: "server not initialized"} }

func readMessage(r *bufio.Reader) ([]byte, error) {
	var out []byte
	for {
		part, err := r.ReadSlice('\n')
		if len(out)+len(part) > MaxMessageBytes {
			return nil, fmt.Errorf("MCP message exceeds %d byte limit", MaxMessageBytes)
		}
		out = append(out, part...)
		if err == nil {
			return out, nil
		}
		if errors.Is(err, bufio.ErrBufferFull) {
			continue
		}
		if errors.Is(err, io.EOF) && len(out) > 0 {
			return out, nil
		}
		return nil, err
	}
}

func decodeID(raw json.RawMessage) any {
	var id any
	if len(raw) == 0 {
		return nil
	}
	_ = json.Unmarshal(raw, &id)
	return id
}

func writeResponse(w *bufio.Writer, response rpcResponse) error {
	data, err := json.Marshal(response)
	if err != nil {
		return err
	}
	if _, err := w.Write(append(data, '\n')); err != nil {
		return err
	}
	return w.Flush()
}
