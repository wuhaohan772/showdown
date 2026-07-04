package agent

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestCustomAdapterViaEnv(t *testing.T) {
	t.Setenv("SHOWDOWN_AGENT_CMD", "testdata/stub.sh")
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
