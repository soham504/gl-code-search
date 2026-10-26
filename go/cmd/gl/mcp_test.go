package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestServeMCP(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "server.py"), []byte("def handle_request(req):\n    pass\n"), 0o644)
	in := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"symbol","arguments":{"name":"handle_request"}}}`,
		`{"jsonrpc":"2.0","id":4,"method":"nope"}`,
		`not json`,
	}, "\n") + "\n"
	var out bytes.Buffer
	if err := serveMCP(strings.NewReader(in), &out, root, false); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 5 { // the notification gets no reply
		t.Fatalf("got %d responses, want 5:\n%s", len(lines), out.String())
	}
	var call struct {
		Result struct {
			Content []struct{ Text string } `json:"content"`
			IsError bool                    `json:"isError"`
		} `json:"result"`
	}
	json.Unmarshal([]byte(lines[2]), &call)
	if call.Result.IsError || !strings.Contains(call.Result.Content[0].Text, "server.py:1 def handle_request") {
		t.Errorf("symbol tool result = %+v", call.Result)
	}
	if !strings.Contains(lines[3], "-32601") || !strings.Contains(lines[4], "-32700") {
		t.Errorf("missing error responses: %s / %s", lines[3], lines[4])
	}
}
