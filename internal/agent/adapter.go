package agent

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Adapter knows how to invoke one agent CLI (the Agent Adapter of CONTEXT.md).
type Adapter struct {
	Key         string
	DisplayName string
	Bin         string
	Model       string   // empty = CLI default
	Models      []string // suggested presets for the picker; empty = flag-only
	Args        func(model, prompt string) []string
	Parse       func(raw string) Response // nil = plain-text output
	Env         []string                  // extra env for the CLI process
	// SessionArgs builds the CLI args for a persistent stream-json session
	// (P2). nil = adapter is stateless-only.
	SessionArgs func(model string) []string
}

// claudeSystemPrompt replaces Claude Code's default system prompt (~8k
// tokens); with --disallowedTools '*' dropping tool schemas (~11k more),
// a decision call shrinks from ~20k to ~2k context tokens. The opponent
// only ever answers with text/JSON, so both are dead weight. Must stay a
// static string: an identical prefix every call keeps the prompt cache
// warm. CLAUDE.md/memory still load via dynamic sections (ADR-0002).
const claudeSystemPrompt = "You are a poker-playing AI opponent in a terminal game. " +
	"Follow the prompt's instructions exactly. Be terse: output ONLY what the prompt " +
	"asks for — no preamble, no explanation, no markdown fences, no thinking out loud."

// claudeSettings disables the player's hooks in the opponent's session.
// User-level SessionStart hooks inject style directives into every claude
// session (seen live: a "caveman mode" plugin turned all table talk into
// broken English). --setting-sources "" would also drop CLAUDE.md memory,
// which ADR-0002 needs for personal needling; disableAllHooks keeps memory
// and removes only the hook injections.
const claudeSettings = `{"disableAllHooks":true}`

var knownAdapters = []Adapter{
	{Key: "claude", DisplayName: "Claude Code", Bin: "claude",
		// Sonnet default: haiku garbles table-talk facts (blames the human
		// for its own folds ~50% of the time; sonnet 0/9 on the same
		// prompts) and costs only ~6x more stateless ($0.015 vs $0.0025
		// per decision). Menu "(default)" and an empty --model keep this.
		Model:  "sonnet",
		Models: []string{"haiku", "sonnet", "opus"},
		Args: func(m, p string) []string {
			args := []string{"-p", p, "--output-format", "json",
				"--disallowedTools", "*",
				"--settings", claudeSettings,
				"--system-prompt", claudeSystemPrompt}
			if m != "" {
				args = append(args, "--model", m)
			}
			return args
		},
		Parse: ParseClaudeJSON,
		// The poker prompt's "think about pot odds" triggers extended
		// thinking: ~1k hidden tokens per decision, 40s+ decode — past the
		// 45s timeout, so every call died to fallback. Thinking off →
		// ~1-2s decisions and the "say" line survives.
		Env: []string{"MAX_THINKING_TOKENS=0"},
		SessionArgs: func(m string) []string {
			args := []string{"-p", "--verbose",
				"--input-format", "stream-json", "--output-format", "stream-json",
				"--disallowedTools", "*",
				"--settings", claudeSettings,
				"--system-prompt", claudeSystemPrompt}
			if m != "" {
				args = append(args, "--model", m)
			}
			return args
		}},
	{Key: "codex", DisplayName: "Codex", Bin: "codex",
		Args: func(m, p string) []string {
			args := []string{"exec"}
			if m != "" {
				args = append(args, "-m", m)
			}
			return append(args, p)
		}},
	{Key: "gemini", DisplayName: "Gemini CLI", Bin: "gemini",
		Args: func(m, p string) []string {
			var args []string
			if m != "" {
				args = append(args, "-m", m)
			}
			return append(args, "-p", p)
		}},
}

// Asker runs the CLI with cwd=dir so it loads its own memory files (ADR-0002).
func (a Adapter) Asker(dir string, timeout time.Duration) Asker {
	return func(ctx context.Context, prompt string) (Response, error) {
		cctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		cmd := exec.CommandContext(cctx, a.Bin, a.Args(a.Model, prompt)...)
		cmd.Dir = dir
		if len(a.Env) > 0 {
			cmd.Env = append(os.Environ(), a.Env...)
		}
		// Guard against a grandchild process holding the stdout pipe open: on
		// context cancellation, exec only signals the direct child, and
		// cmd.Output() would otherwise block forever waiting for stdout to
		// close. WaitDelay bounds that wait, then kills the process group.
		cmd.WaitDelay = 5 * time.Second
		out, err := cmd.Output()
		if err != nil {
			return Response{}, err
		}
		if a.Parse != nil {
			return a.Parse(string(out)), nil
		}
		return Response{Text: string(out)}, nil
	}
}

// DetectRoster finds installed agent CLIs. SHOWDOWN_AGENT_CMD overrides
// everything with a custom command (prompt appended as final arg) — test hook.
func DetectRoster() []Adapter {
	if custom := os.Getenv("SHOWDOWN_AGENT_CMD"); custom != "" {
		parts := strings.Fields(custom)
		// Whitespace-only commands fall through to normal PATH detection; no shell-quoting support, space-separated tokens only.
		if len(parts) == 0 {
			// Fall through to normal PATH detection
		} else {
			return []Adapter{{
				Key: "custom", DisplayName: parts[0], Bin: parts[0],
				Args: func(_, p string) []string { return append(append([]string{}, parts[1:]...), p) },
			}}
		}
	}
	var out []Adapter
	for _, a := range knownAdapters {
		if _, err := exec.LookPath(a.Bin); err == nil {
			out = append(out, a)
		}
	}
	return out
}

func (a Adapter) SupportsSession() bool { return a.SessionArgs != nil }

// StartSession launches a persistent session for this adapter with
// cwd=dir (ADR-0002) and the adapter's Env, mirroring Asker.
func (a Adapter) StartSession(dir string, timeout time.Duration) (*Session, error) {
	if a.SessionArgs == nil {
		return nil, fmt.Errorf("adapter %s does not support sessions", a.Key)
	}
	return StartSession(a.Bin, a.SessionArgs(a.Model), a.Env, dir, timeout)
}
