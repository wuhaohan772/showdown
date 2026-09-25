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
	return mascotModel(t, "claude", "Claude Code")
}

func mascotModel(t *testing.T, key, name string) Model {
	t.Helper()
	ad := agent.Adapter{Key: key, DisplayName: name, Bin: "true",
		Args: func(m, p string) []string { return nil }}
	m := NewModel(ad, stats.Stats{}, t.TempDir()+"/stats.json", false, ".", "p", 1500, 10, 0, nil)
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	m = m2.(Model)
	m2, _ = m.Update(startHandMsg{})
	return m2.(Model)
}

func TestClawdRendersForClaude(t *testing.T) {
	t.Setenv("SHOWDOWN_REDUCE_MOTION", "1")
	view := stripANSI(claudeModel(t).View())
	for _, row := range mascots["claude"].art("default") {
		if !strings.Contains(view, strings.TrimRight(row, " ")) {
			t.Errorf("clawd row %q missing from the claude table:\n%s", row, view)
		}
	}
	if !strings.Contains(view, "Claude Code") {
		t.Error("opponent name must sit beside the mascot")
	}
	if !strings.Contains(view, "stack: 1480") {
		t.Error("opponent stack must sit beside the mascot")
	}

	// a non-claude opponent keeps the plain one-line header
	p2, _ := testModel(t).Update(startHandMsg{})
	plain := stripANSI(p2.(Model).View())
	if strings.Contains(plain, "▐▛███▛█") {
		t.Error("mascot must not render for non-claude opponents")
	}
	if !strings.Contains(plain, "♠ Stub   stack:") {
		t.Error("non-claude opponent lost its header line")
	}
}

func TestMascotPoseFollowsTable(t *testing.T) {
	if got := mascotPoseFor(false, false, 0); got != "default" {
		t.Errorf("idle pose = %q", got)
	}
	if got := mascotPoseFor(true, false, 0); got != "look-left" {
		t.Errorf("thinking pose at frame 0 = %q", got)
	}
	if got := mascotPoseFor(true, false, 6); got != "look-right" {
		t.Errorf("thinking pose at frame 6 = %q, want the eyes to have moved", got)
	}
	if got := mascotPoseFor(true, true, 0); got != "arms-up" {
		t.Errorf("winning pose = %q", got)
	}
	// every mascot draws every pose, and an unknown pose still draws something
	for key, mc := range mascots {
		for _, pose := range []string{"default", "look-left", "look-right", "arms-up", "no-such-pose"} {
			if rows := mc.art(pose); len(rows) != 3 || rows[0] == "" {
				t.Errorf("%s pose %q: want three drawable rows, got %q", key, pose, rows)
			}
		}
	}
}

// The header copies Claude Code's banner: text at column 11 of the art box,
// one line per art row, and a blank row above it.
func TestMascotHeaderMatchesClaudeCodeBanner(t *testing.T) {
	t.Setenv("SHOWDOWN_REDUCE_MOTION", "1")
	lines := strings.Split(stripANSI(claudeModel(t).View()), "\n")
	if strings.TrimSpace(strings.Split(lines[0], "│")[0]) != "" {
		t.Errorf("first row must be blank, got %q", lines[0])
	}
	want := []string{
		"   ▐▛███▛█   Claude Code",
		"  ▝▜██████▀  default model",
		"   ▝▝   ▝▝   stack: 1480",
	}
	for i, w := range want {
		if got := strings.TrimRight(strings.Split(lines[1+i], "│")[0], " "); got != w {
			t.Errorf("row %d = %q, want %q", 1+i, got, w)
		}
	}
}

func TestCodexGetsPromptBot(t *testing.T) {
	t.Setenv("SHOWDOWN_REDUCE_MOTION", "1")
	view := stripANSI(mascotModel(t, "codex", "Codex").View())
	for _, row := range mascots["codex"].art("default") {
		if !strings.Contains(view, strings.TrimRight(row, " ")) {
			t.Errorf("codex row %q missing:\n%s", row, view)
		}
	}
	if strings.Contains(view, "▐▛███▛█") {
		t.Error("codex must not get clawd")
	}
}

func click(m Model, x, y int) Model {
	m2, _ := m.Update(tea.MouseMsg{X: x, Y: y, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
	return m2.(Model)
}

func TestClickingMascotPlaysReaction(t *testing.T) {
	m := click(claudeModel(t), mascotLeft+4, mascotTop+1)
	if m.clickAnim == nil {
		t.Fatal("click on the mascot must start a reaction")
	}
	if !m.animRunning {
		t.Error("reaction must start the animation ticker")
	}
	m2, _ := m.Update(animTickMsg{})
	m = m2.(Model)
	if m = click(m, mascotLeft+4, mascotTop+1); m.clickIdx != 1 {
		t.Error("a click mid-reaction must not restart it")
	}
	for i := 0; i < 40 && m.clickAnim != nil; i++ {
		m2, _ := m.Update(animTickMsg{})
		m = m2.(Model)
	}
	if m.clickAnim != nil {
		t.Error("reaction must end")
	}

	if m := click(claudeModel(t), 40, 10); m.clickAnim != nil {
		t.Error("a click off the mascot must do nothing")
	}
	t.Setenv("SHOWDOWN_REDUCE_MOTION", "1")
	if m := click(claudeModel(t), mascotLeft+4, mascotTop+1); m.clickAnim != nil {
		t.Error("reduced motion must ignore clicks")
	}
}

func TestCrouchFrameDropsArtWithPoof(t *testing.T) {
	out := stripANSI(mascots["claude"].render(mascotFrame{pose: "default", offset: 1, poof: "~"}, "T", "M", "S", nil))
	lines := strings.Split(out, "\n")
	if strings.ContainsAny(lines[0], "▐▛▜▝") {
		t.Errorf("crouch must leave the top row empty, got %q", lines[0])
	}
	if !strings.Contains(lines[1], "▐▛███▛█") {
		t.Errorf("crouch row 1 = %q, want the head", lines[1])
	}
	if !strings.Contains(lines[2], "~▜██████~") {
		t.Errorf("crouch row 2 = %q, want the body flanked by poof", lines[2])
	}
	// the Codex box keeps its border; the poof sits outside it
	out = stripANSI(mascots["codex"].render(mascotFrame{pose: "default", offset: 1, poof: "~"}, "T", "M", "S", nil))
	if !strings.Contains(out, "~│ >_  │~") {
		t.Errorf("codex crouch must keep its border:\n%s", out)
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

func TestSelectHintOnlyWhenMouseCaptured(t *testing.T) {
	t.Setenv("SHOWDOWN_REDUCE_MOTION", "1")
	t.Setenv("TERM_PROGRAM", "iTerm.app")
	if view := stripANSI(claudeModel(t).View()); strings.Contains(view, "while dragging") {
		t.Error("no hint when the mouse is not captured")
	}
	view := stripANSI(claudeModel(t).WithMouse(true).View())
	if !strings.Contains(view, "hold ⌥ option while dragging to highlight text") {
		t.Errorf("mouse hint missing:\n%s", view)
	}
	for term, want := range map[string]string{"Apple_Terminal": "fn", "iTerm.app": "⌥ option", "ghostty": "shift", "": "shift"} {
		if got := selectKey(term); got != want {
			t.Errorf("selectKey(%q) = %q, want %q", term, got, want)
		}
	}
}
