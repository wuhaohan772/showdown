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
	Args        func(prompt string) []string
}

var knownAdapters = []Adapter{
	{Key: "claude", DisplayName: "Claude Code", Bin: "claude",
		Args: func(p string) []string { return []string{"-p", p} }},
	{Key: "codex", DisplayName: "Codex", Bin: "codex",
		Args: func(p string) []string { return []string{"exec", p} }},
	{Key: "gemini", DisplayName: "Gemini CLI", Bin: "gemini",
		Args: func(p string) []string { return []string{"-p", p} }},
}

// Asker runs the CLI with cwd=dir so it loads its own memory files (ADR-0002).
func (a Adapter) Asker(dir string, timeout time.Duration) Asker {
	return func(ctx context.Context, prompt string) (string, error) {
		cctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		cmd := exec.CommandContext(cctx, a.Bin, a.Args(prompt)...)
		cmd.Dir = dir
		out, err := cmd.Output()
		if err != nil {
			return "", err
		}
		return string(out), nil
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
				Args: func(p string) []string { return append(append([]string{}, parts[1:]...), p) },
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
