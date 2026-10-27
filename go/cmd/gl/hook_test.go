package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func tree(t *testing.T) string {
	root := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	os.MkdirAll(filepath.Join(root, "app"), 0o755)
	os.WriteFile(filepath.Join(root, "app/billing.py"), []byte("def compute_invoice_total(items):\n    return sum(items)\n"), 0o644)
	os.WriteFile(filepath.Join(root, "app/users.py"), []byte("def create_user(name):\n    return name\n"), 0o644)
	return root
}

func hookEvent(t *testing.T, root, prompt string) string {
	ev, _ := json.Marshal(map[string]string{"prompt": prompt, "cwd": root, "hook_event_name": "UserPromptSubmit"})
	var out bytes.Buffer
	runHook(bytes.NewReader(ev), &out, ".", 10)
	return out.String()
}

func TestHookInjectsWhenConfident(t *testing.T) {
	root := tree(t)
	out := hookEvent(t, root, "Invoices are wrong: `compute_invoice_total` returns the wrong value when items is empty, please fix")
	var resp struct {
		HookSpecificOutput struct {
			HookEventName     string `json:"hookEventName"`
			AdditionalContext string `json:"additionalContext"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("hook output is not JSON: %q", out)
	}
	ctx := resp.HookSpecificOutput.AdditionalContext
	if resp.HookSpecificOutput.HookEventName != "UserPromptSubmit" || !strings.Contains(ctx, "1. app/billing.py") {
		t.Errorf("unexpected hook output: %+v", resp)
	}
}

func TestHookStaysSilent(t *testing.T) {
	root := tree(t)
	if out := hookEvent(t, root, "hi"); out != "" {
		t.Errorf("short prompt should inject nothing, got %q", out)
	}
	if out := hookEvent(t, root, "can you explain how this project is organised and what the main ideas are"); out != "" {
		t.Errorf("prompt without strong evidence should inject nothing, got %q", out)
	}
	var out bytes.Buffer
	runHook(strings.NewReader("not json"), &out, ".", 10)
	if out.Len() != 0 {
		t.Errorf("bad input must be ignored, got %q", out.String())
	}
}

func TestResidentIndexRebuildsOnlyOnChange(t *testing.T) {
	root := tree(t)
	r := &resident{root: root}
	ix1, _, err := r.index()
	if err != nil {
		t.Fatal(err)
	}
	ix2, _, _ := r.index()
	if ix1 != ix2 {
		t.Error("unchanged tree should reuse the resident index")
	}
	time.Sleep(10 * time.Millisecond)
	os.WriteFile(filepath.Join(root, "app/refunds.py"), []byte("def issue_refund(): pass\n"), 0o644)
	ix3, _, _ := r.index()
	if ix3 == ix2 || len(ix3.Definitions("issue_refund")) != 1 {
		t.Error("new file should trigger a rebuild that includes it")
	}
}
