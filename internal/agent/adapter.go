package agent

import (
	"context"
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
}

var knownAdapters = []Adapter{
	{Key: "claude", DisplayName: "Claude Code", Bin: "claude",
		Models: []string{"haiku", "sonnet", "opus"},
		Args: func(m, p string) []string {
			args := []string{"-p", p, "--output-format", "json"}
			if m != "" {
				args = append(args, "--model", m)
			}
			return args
		},
		Parse: ParseClaudeJSON},
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
