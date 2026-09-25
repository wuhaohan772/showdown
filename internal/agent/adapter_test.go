package agent

import (
	"context"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// stubCmd is the canned-decision agent used by the adapter tests. Windows
// cannot exec the shell script, so it gets the batch twin.
func stubCmd() string {
	if runtime.GOOS == "windows" {
		return filepath.Join("testdata", "stub.bat") // cmd.exe rejects forward slashes in a .bat path
	}
	return "testdata/stub.sh"
}

func TestCustomAdapterViaEnv(t *testing.T) {
	t.Setenv("SHOWDOWN_AGENT_CMD", stubCmd())
	roster := DetectRoster()
	if len(roster) != 1 || roster[0].Key != "custom" {
		t.Fatalf("roster = %+v, want single custom adapter", roster)
	}
	ask := roster[0].Asker(".", 10*time.Second)
	out, err := ask(context.Background(), "PROMPT")
	if err != nil {
		t.Fatalf("ask: %v", err)
	}
	if !strings.Contains(out.Text, `"action": "call"`) {
		t.Errorf("out.Text = %q", out.Text)
	}
	if out.Usage != nil {
		t.Errorf("custom adapter Usage = %+v, want nil", out.Usage)
	}
}

func TestAskerTimeout(t *testing.T) {
	t.Setenv("SHOWDOWN_AGENT_CMD", "sleep")
	roster := DetectRoster()
	ask := roster[0].Asker(".", 200*time.Millisecond)
	start := time.Now()
	_, err := ask(context.Background(), "5") // sleep 5 — must be killed
	if err == nil {
		t.Fatal("want timeout error")
	}
	if time.Since(start) > 2*time.Second {
		t.Error("timeout did not kill the process promptly")
	}
}

func TestKnownAdapterArgs(t *testing.T) {
	for _, a := range knownAdapters {
		for _, model := range []string{"", "test-model"} {
			args := a.Args(model, "PROMPT")
			joined := strings.Join(args, " ")
			if !strings.Contains(joined, "PROMPT") {
				t.Errorf("%s(model=%q): prompt not in args %v", a.Key, model, args)
			}
			if model != "" && !strings.Contains(joined, "test-model") {
				t.Errorf("%s: model not in args %v", a.Key, args)
			}
			if model == "" && strings.Contains(joined, "--model") || model == "" && strings.Contains(joined, "-m ") {
				t.Errorf("%s: model flag present with empty model: %v", a.Key, args)
			}
		}
	}
}

func TestClaudeAdapterHasModelPresets(t *testing.T) {
	for _, a := range knownAdapters {
		if a.Key == "claude" && len(a.Models) == 0 {
			t.Error("claude adapter should suggest model presets")
		}
	}
}

// Sonnet is the claude default (2026-07-04): haiku flips fold-attribution
// in table talk ~50% after its own fold; sonnet was 0/9 on the same prompts.
func TestClaudeAdapterDefaultsToSonnet(t *testing.T) {
	for _, a := range knownAdapters {
		if a.Key == "claude" && a.Model != "sonnet" {
			t.Errorf("claude default model = %q, want sonnet", a.Model)
		}
	}
}

func TestWhitespaceCustomCmdFallsBack(t *testing.T) {
	t.Setenv("SHOWDOWN_AGENT_CMD", "   ")
	roster := DetectRoster() // must not panic
	for _, a := range roster {
		if a.Key == "custom" {
			t.Errorf("whitespace-only SHOWDOWN_AGENT_CMD produced custom adapter")
		}
	}
}

func TestClaudeAdapterDisablesThinking(t *testing.T) {
	for _, a := range knownAdapters {
		if a.Key != "claude" {
			continue
		}
		for _, e := range a.Env {
			if e == "MAX_THINKING_TOKENS=0" {
				return
			}
		}
		t.Fatalf("claude adapter Env = %v, want MAX_THINKING_TOKENS=0", a.Env)
	}
	t.Fatal("no claude adapter in knownAdapters")
}

func TestAskerAppliesEnv(t *testing.T) {
	a := Adapter{Key: "x", Bin: "sh",
		Env:  []string{"SHOWDOWN_TEST_ENV=marker"},
		Args: func(m, p string) []string { return []string{"-c", `printf %s "$SHOWDOWN_TEST_ENV"`} }}
	resp, err := a.Asker(".", 10*time.Second)(context.Background(), "ignored")
	if err != nil {
		t.Fatalf("ask: %v", err)
	}
	if resp.Text != "marker" {
		t.Errorf("Text = %q, want env var passed to child", resp.Text)
	}
}

func TestClaudeAdapterLeanSession(t *testing.T) {
	for _, a := range knownAdapters {
		if a.Key != "claude" {
			continue
		}
		args := a.Args("", "PROMPT")
		joined := strings.Join(args, "\x00")
		if !strings.Contains(joined, "--disallowedTools\x00*") {
			t.Errorf("claude args must disallow all tools, got %v", args)
		}
		if !strings.Contains(joined, "--settings\x00"+`{"disableAllHooks":true}`) {
			t.Errorf("claude args must disable the player's hooks, got %v", args)
		}
		sysIdx := -1
		for i, x := range args {
			if x == "--system-prompt" {
				sysIdx = i
			}
		}
		if sysIdx == -1 || sysIdx+1 >= len(args) || args[sysIdx+1] == "" {
			t.Errorf("claude args must carry a non-empty --system-prompt, got %v", args)
		}
		return
	}
	t.Fatal("no claude adapter in knownAdapters")
}

func TestClaudeAdapterUsesJSONOutput(t *testing.T) {
	for _, a := range knownAdapters {
		if a.Key != "claude" {
			continue
		}
		args := strings.Join(a.Args("", "PROMPT"), " ")
		if !strings.Contains(args, "--output-format json") {
			t.Errorf("claude args = %q, want --output-format json", args)
		}
		if a.Parse == nil {
			t.Error("claude adapter must set Parse")
		}
		return
	}
	t.Fatal("no claude adapter in knownAdapters")
}

func TestAskerAppliesParse(t *testing.T) {
	a := Adapter{Key: "x", Bin: "echo",
		Args:  func(m, p string) []string { return []string{sampleEnvelope} },
		Parse: ParseClaudeJSON}
	resp, err := a.Asker(".", 10*time.Second)(context.Background(), "ignored")
	if err != nil {
		t.Fatalf("ask: %v", err)
	}
	if resp.Text != `{"action": "call"}` {
		t.Errorf("Text = %q, want unwrapped result", resp.Text)
	}
	if resp.Usage == nil || resp.Usage.OutputTokens != 479 {
		t.Errorf("Usage = %+v", resp.Usage)
	}
}

func TestClaudeSessionArgs(t *testing.T) {
	var claude Adapter
	for _, a := range knownAdapters {
		if a.Key == "claude" {
			claude = a
		} else if a.SessionArgs != nil {
			t.Errorf("%s must not support sessions", a.Key)
		}
	}
	if !claude.SupportsSession() {
		t.Fatal("claude must support sessions")
	}
	args := claude.SessionArgs("sonnet")
	joined := strings.Join(args, " ")
	for _, w := range []string{
		"-p", "--verbose",
		"--input-format stream-json", "--output-format stream-json",
		"--disallowedTools *", "--system-prompt", "--model sonnet",
		`--settings {"disableAllHooks":true}`,
	} {
		if !strings.Contains(joined, w) {
			t.Errorf("session args missing %q in %q", w, joined)
		}
	}
	// The --system-prompt VALUE must be non-empty and match the prefix of
	// claudeSystemPrompt so the prompt cache stays warm across turns.
	sysIdx := -1
	for i, a := range args {
		if a == "--system-prompt" {
			sysIdx = i
			break
		}
	}
	if sysIdx == -1 || sysIdx+1 >= len(args) {
		t.Error("session args: --system-prompt flag not found or has no value")
	} else if args[sysIdx+1] == "" {
		t.Error("session args: --system-prompt value is empty")
	} else if !strings.HasPrefix(args[sysIdx+1], "You are a poker") {
		t.Errorf("session args: --system-prompt value = %q, want prefix matching claudeSystemPrompt", args[sysIdx+1])
	}
	if strings.Contains(strings.Join(claude.SessionArgs(""), " "), "--model") {
		t.Error("empty model must not add --model")
	}
}

func TestStartSessionUnsupportedAdapter(t *testing.T) {
	a := Adapter{Key: "codex", Bin: "true"}
	if _, err := a.StartSession(t.TempDir(), time.Second); err == nil {
		t.Error("StartSession on session-less adapter must error")
	}
}

// Codex defaults to its mid tier at low effort, matching claude's sonnet,
// and the effort holds even when the player picks another model.
func TestCodexDefaultsMatchSonnet(t *testing.T) {
	var codex Adapter
	for _, a := range knownAdapters {
		if a.Key == "codex" {
			codex = a
		}
	}
	if codex.Model != "gpt-6-sol" {
		t.Errorf("codex default model = %q, want gpt-6-sol", codex.Model)
	}
	for _, m := range []string{codex.Model, "gpt-6-astra"} {
		got := strings.Join(codex.Args(m, "PROMPT"), " ")
		want := `exec -c model_reasoning_effort="low" -m ` + m + " PROMPT"
		if got != want {
			t.Errorf("codex args = %q, want %q", got, want)
		}
	}
}
