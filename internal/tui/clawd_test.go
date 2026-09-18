package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/wuhaohan772/showdown/internal/agent"
	"github.com/wuhaohan772/showdown/internal/stats"
)

func claudeModel(t *testing.T) Model {
	t.Helper()
	ad := agent.Adapter{Key: "claude", DisplayName: "Claude Code", Bin: "true",
		Args: func(m, p string) []string { return nil }}
	m := NewModel(ad, stats.Stats{}, t.TempDir()+"/stats.json", false, ".", "p", 1500, 10, 0, nil)
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	m = m2.(Model)
	m2, _ = m.Update(startHandMsg{})
	return m2.(Model)
}

func TestClawdRendersForClaudeOnly(t *testing.T) {
	t.Setenv("SHOWDOWN_REDUCE_MOTION", "1")
	view := stripANSI(claudeModel(t).View())
	for _, row := range clawdArt("default") {
		if !strings.Contains(view, strings.TrimRight(row, " ")) {
			t.Errorf("clawd row %q missing from the claude table:\n%s", row, view)
		}
	}
	if !strings.Contains(view, "♠ Claude Code") {
		t.Error("opponent name must sit beside the mascot")
	}
	if !strings.Contains(view, "stack: 1480") {
		t.Error("opponent stack must sit beside the mascot")
	}

	// a non-claude opponent keeps the plain one-line header
	p2, _ := testModel(t).Update(startHandMsg{})
	plain := stripANSI(p2.(Model).View())
	if strings.Contains(plain, "▛███") {
		t.Error("mascot must not render for non-claude opponents")
	}
	if !strings.Contains(plain, "♠ Stub   stack:") {
		t.Error("non-claude opponent lost its header line")
	}
}

func TestClawdPoseFollowsTable(t *testing.T) {
	if got := clawdPoseFor(false, false, 0); got != "default" {
		t.Errorf("idle pose = %q", got)
	}
	if got := clawdPoseFor(true, false, 0); got != "look-left" {
		t.Errorf("thinking pose at frame 0 = %q", got)
	}
	if got := clawdPoseFor(true, false, 6); got != "look-right" {
		t.Errorf("thinking pose at frame 6 = %q, want the eyes to have moved", got)
	}
	if got := clawdPoseFor(true, true, 0); got != "arms-up" {
		t.Errorf("winning pose = %q", got)
	}
	// an unknown pose still draws something
	if len(clawdArt("no-such-pose")) != 3 {
		t.Error("unknown pose must fall back to three drawable rows")
	}
}

func TestClawdThinkingLineNotDoubled(t *testing.T) {
	t.Setenv("SHOWDOWN_REDUCE_MOTION", "1")
	m := claudeModel(t)
	m.phase = phaseAgentTurn
	view := stripANSI(m.View())
	if n := strings.Count(view, "thinking..."); n != 1 {
		t.Errorf("thinking line appears %d times, want 1:\n%s", n, view)
	}
	if strings.Contains(view, "Claude Code is thinking") {
		t.Error("clawd block should carry the short thinking line, not the named one")
	}
}
