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

func TestHandSummarySplitPot(t *testing.T) {
	r := &poker.Result{Split: true, Pot: 40, Desc: "chopped, both had two pair"}
	got := handSummary(3, r, "YOU")
	want := "Hand 3: split pot (40) — chopped, both had two pair."
	if got != want {
		t.Errorf("handSummary = %q, want %q", got, want)
	}
}

func TestHandSummaryDecisiveWin(t *testing.T) {
	r := &poker.Result{Showdown: true, Pot: 80, Desc: "a pair of kings"}
	got := handSummary(5, r, "Stub")
	want := "Hand 5: Stub won 80 (a pair of kings)."
	if got != want {
		t.Errorf("handSummary = %q, want %q", got, want)
	}
	r2 := &poker.Result{Showdown: false, Pot: 20}
	got2 := handSummary(6, r2, "YOU")
	want2 := "Hand 6: YOU won 20 (opponent folded)."
	if got2 != want2 {
		t.Errorf("handSummary = %q, want %q", got2, want2)
	}
}

// TestMatchOverShowsCorrectStacks guards against a regression where the
// match-over screen rendered stacks/hole cards using the seat mapping for
// the *next* hand (humanSeat()/agentSeat() flip once NextHand advances
// HandNum), even though it was still displaying the just-finished hand.
// That bug showed "YOU stack: 0" even when the human won the match.
func TestMatchOverShowsCorrectStacks(t *testing.T) {
	m := testModel(t)
	m2, _ := m.Update(startHandMsg{})
	m = m2.(Model)

	// Simulate the human winning the final hand outright: human's seat ends
	// with the whole 3000-chip pool, agent's seat is busted.
	hs, as := m.humanSeat(), m.agentSeat()
	m.hand.Seats[hs].Stack = 3000
	m.hand.Seats[as].Stack = 0

	m.settleAndNext()

	if m.phase != phaseMatchOver {
		t.Fatalf("phase = %v, want phaseMatchOver", m.phase)
	}
	view := stripANSI(m.View())
	if !strings.Contains(view, "YOU   stack: 3000") {
		t.Errorf("view should show human's winning stack 3000 on the YOU line, got:\n%s", view)
	}
	if strings.Contains(view, "YOU   stack: 0") {
		t.Errorf("view incorrectly shows YOU stack: 0 (inverted result), got:\n%s", view)
	}
}

func TestFallbackNoticeVisibleInQuietMode(t *testing.T) {
	ad := agent.Adapter{Key: "stub", DisplayName: "Stub", Bin: "true",
		Args: func(p string) []string { return nil }}
	m := NewModel(ad, stats.Stats{}, t.TempDir()+"/stats.json", true, ".")
	m2, _ := m.Update(startHandMsg{})
	m = m2.(Model)
	m2, _ = m.Update(key("c")) // human limps; agent (BB) has the option
	m = m2.(Model)
	if m.phase != phaseAgentTurn {
		t.Fatalf("phase = %v, want phaseAgentTurn", m.phase)
	}
	m2, _ = m.Update(decisionMsg{act: poker.Action{Type: poker.Check}, fallback: true})
	m = m2.(Model)
	if !strings.Contains(stripANSI(m.View()), "agent glitched") {
		t.Errorf("view = %q, want it to contain the glitch notice even in quiet mode", stripANSI(m.View()))
	}
}
