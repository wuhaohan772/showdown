package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/haohanwu/showdown/internal/agent"
	"github.com/haohanwu/showdown/internal/poker"
	"github.com/haohanwu/showdown/internal/stats"
)

func testModel(t *testing.T) Model {
	t.Helper()
	ad := agent.Adapter{Key: "stub", DisplayName: "Stub", Bin: "true",
		Args: func(p string) []string { return nil }}
	return NewModel(ad, stats.Stats{}, t.TempDir()+"/stats.json", false, ".")
}

func key(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func TestHandStartsWithHumanOrAgentTurn(t *testing.T) {
	m := testModel(t)
	m2, _ := m.Update(startHandMsg{})
	m = m2.(Model)
	// hand 1: human is button (seat 0) and acts first preflop
	if m.phase != phaseHumanTurn {
		t.Fatalf("phase = %v, want phaseHumanTurn", m.phase)
	}
	if !strings.Contains(stripANSI(m.View()), "Stub") {
		t.Error("view should show opponent name")
	}
}

func TestHumanFoldEndsHand(t *testing.T) {
	m := testModel(t)
	m2, _ := m.Update(startHandMsg{})
	m = m2.(Model)
	m2, _ = m.Update(key("f"))
	m = m2.(Model)
	if m.phase != phaseHandEnd {
		t.Fatalf("phase = %v, want phaseHandEnd", m.phase)
	}
	if m.hand.Result() == nil || m.hand.Result().Winner != 1 {
		t.Error("agent seat should win when human folds")
	}
}

func TestAgentDecisionMsgApplies(t *testing.T) {
	m := testModel(t)
	m2, _ := m.Update(startHandMsg{})
	m = m2.(Model)
	m2, _ = m.Update(key("c")) // human limps; agent (BB) has the option
	m = m2.(Model)
	if m.phase != phaseAgentTurn {
		t.Fatalf("phase = %v, want phaseAgentTurn", m.phase)
	}
	m2, _ = m.Update(decisionMsg{act: poker.Action{Type: poker.Check}, say: "hm"})
	m = m2.(Model)
	if m.hand.Street != poker.Flop {
		t.Errorf("street = %v, want Flop after limp+check", m.hand.Street)
	}
	if m.agentSay != "hm" {
		t.Errorf("agentSay = %q", m.agentSay)
	}
}

func TestRaiseInputFlow(t *testing.T) {
	m := testModel(t)
	m2, _ := m.Update(startHandMsg{})
	m = m2.(Model)
	m2, _ = m.Update(key("r"))
	m = m2.(Model)
	if m.phase != phaseRaiseInput {
		t.Fatalf("phase = %v, want phaseRaiseInput", m.phase)
	}
	for _, r := range "60" {
		m2, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = m2.(Model)
	}
	m2, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = m2.(Model)
	if m.phase != phaseAgentTurn {
		t.Fatalf("phase = %v, want phaseAgentTurn after raise", m.phase)
	}
	if m.hand.CurrentBet != 60 {
		t.Errorf("CurrentBet = %d, want 60", m.hand.CurrentBet)
	}
}

func TestTalkInputFeedsDigest(t *testing.T) {
	m := testModel(t)
	m2, _ := m.Update(startHandMsg{})
	m = m2.(Model)
	m2, _ = m.Update(key("t"))
	m = m2.(Model)
	for _, r := range "ez" {
		m2, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = m2.(Model)
	}
	m2, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = m2.(Model)
	if !strings.Contains(m.digest.HandTalk(), "HUMAN: ez") {
		t.Errorf("talk = %q", m.digest.HandTalk())
	}
	if m.phase != phaseHumanTurn {
		t.Errorf("phase = %v, want back to phaseHumanTurn", m.phase)
	}
}
