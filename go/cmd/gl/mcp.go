package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/soham504/gl-code-search/go/internal/code"
)

// A minimal Model Context Protocol server over stdio: newline-delimited
// JSON-RPC 2.0 exposing gl's locate/search/symbol as tools. The index stays
// resident: each call re-lists and stats the tree (no file reads) and rebuilds
// only when a file was added, removed or modified, so results always reflect
// files the agent has just edited.

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

func schema(prop, desc string) map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{prop: map[string]any{"type": "string", "description": desc}},
		"required":   []string{prop},
	}
}

var mcpTools = []map[string]any{
	{"name": "locate", "description": "Rank the source files most likely to need changes for a bug report or issue. " +
		"Pass the full issue text (stack traces, error messages and identifiers help). Use this first when you don't know where to look.",
		"inputSchema": schema("issue", "Full text of the issue or bug report")},
	{"name": "search", "description": "Code-aware ranked search: files matching the query words (camelCase/snake_case aware), " +
		"with their best-matching lines. Better than grep for concepts; use grep for exact strings.",
		"inputSchema": schema("query", "Words, identifiers or a short description")},
	{"name": "symbol", "description": "Find where a function, class or type is defined (file:line).",
		"inputSchema": schema("name", "Exact symbol name, e.g. QuerySet or parse_header")},
}

// resident keeps the last index and the file listing it was built from.
type resident struct {
	root  string
	cache *code.Cache
	ix    *code.Index
	sig   string
}

func (r *resident) index() (*code.Index, time.Duration, error) {
	start := time.Now()
	files, err := code.LoadDir(r.root)
	if err != nil {
		return nil, 0, err
	}
	h := sha256.New()
	for _, f := range files {
		fmt.Fprintf(h, "%s\x00%d\x00%d\n", f.Path, f.Size, f.ModTime)
	}
	sig := string(h.Sum(nil))
	if r.ix == nil || sig != r.sig {
		r.ix, r.sig = code.BuildCached(files, r.cache), sig
		r.cache.Save()
	}
	return r.ix, time.Since(start).Round(time.Millisecond), nil
}

func serveMCP(in io.Reader, out io.Writer, root string, useCache bool) error {
	res := &resident{root: root}
	if useCache {
		res.cache = code.OpenCache(root)
	}
	sc := bufio.NewScanner(in)
	sc.Buffer(make([]byte, 1<<20), 64<<20)
	enc := json.NewEncoder(out)
	for sc.Scan() {
		var req rpcRequest
		if err := json.Unmarshal(sc.Bytes(), &req); err != nil {
			enc.Encode(rpcResponse{JSONRPC: "2.0", ID: json.RawMessage("null"), Error: &rpcError{-32700, "parse error"}})
			continue
		}
		if len(req.ID) == 0 { // notification (e.g. notifications/initialized): no reply
			continue
		}
		resp := rpcResponse{JSONRPC: "2.0", ID: req.ID}
		switch req.Method {
		case "initialize":
			var p struct {
				ProtocolVersion string `json:"protocolVersion"`
			}
			json.Unmarshal(req.Params, &p)
			if p.ProtocolVersion == "" {
				p.ProtocolVersion = "2025-06-18"
			}
			resp.Result = map[string]any{
				"protocolVersion": p.ProtocolVersion,
				"capabilities":    map[string]any{"tools": map[string]any{}},
				"serverInfo":      map[string]any{"name": "groundline-gl", "version": version},
			}
		case "ping":
			resp.Result = map[string]any{}
		case "tools/list":
			resp.Result = map[string]any{"tools": mcpTools}
		case "tools/call":
			resp.Result = callTool(req.Params, res)
		default:
			resp.Error = &rpcError{-32601, "method not found: " + req.Method}
		}
		if err := enc.Encode(resp); err != nil {
			return err
		}
	}
	return sc.Err()
}

func callTool(params json.RawMessage, res *resident) map[string]any {
	var p struct {
		Name      string            `json:"name"`
		Arguments map[string]string `json:"arguments"`
	}
	result := func(text string, isErr bool) map[string]any {
		return map[string]any{"content": []map[string]any{{"type": "text", "text": text}}, "isError": isErr}
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return result("invalid arguments: "+err.Error(), true)
	}
	ix, took, err := res.index()
	if err != nil {
		return result("indexing failed: "+err.Error(), true)
	}
	switch p.Name {
	case "locate":
		return result(locateText(ix, p.Arguments["issue"], 10, took), false)
	case "search":
		return result(searchText(ix, p.Arguments["query"], 10), false)
	case "symbol":
		return result(symbolText(ix, p.Arguments["name"]), false)
	}
	return result(fmt.Sprintf("unknown tool %q", p.Name), true)
}
