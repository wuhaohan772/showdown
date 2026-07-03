package tui

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/haohanwu/showdown/internal/agent"
	"github.com/haohanwu/showdown/internal/debuglog"
	"github.com/haohanwu/showdown/internal/poker"
	"github.com/haohanwu/showdown/internal/stats"
)

func testModel(t *testing.T) Model {
	t.Helper()
	ad := agent.Adapter{Key: "stub", DisplayName: "Stub", Bin: "true",
		Args: func(p string) []string { return nil }}
	return NewModel(ad, stats.Stats{}, t.TempDir()+"/stats.json", false, ".", nil)
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

func readLogEvents(t *testing.T, path string) []map[string]any {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open log: %v", err)
	}
	defer f.Close()
	var out []map[string]any
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	for sc.Scan() {
		var m map[string]any
		if err := json.Unmarshal(sc.Bytes(), &m); err != nil {
			t.Fatalf("invalid JSONL: %v", err)
		}
		out = append(out, m)
	}
	return out
}

func TestSessionStartLogged(t *testing.T) {
	path := filepath.Join(t.TempDir(), "d.jsonl")
	l, err := debuglog.New(path)
	if err != nil {
		t.Fatalf("debuglog.New: %v", err)
	}
	ad := agent.Adapter{Key: "stub", DisplayName: "Stub", Bin: "true",
		Args: func(p string) []string { return nil }}
	NewModel(ad, stats.Stats{}, t.TempDir()+"/stats.json", true, ".", l)
	l.Close()

	evs := readLogEvents(t, path)
	if len(evs) != 1 || evs[0]["event"] != "session_start" {
		t.Fatalf("events = %v, want one session_start", evs)
	}
	if evs[0]["agent_key"] != "stub" || evs[0]["quiet"] != true {
		t.Errorf("session_start fields = %v", evs[0])
	}
	if _, ok := evs[0]["seed"]; !ok {
		t.Error("session_start missing seed")
	}
}

func TestDebugLogCapturesHandFlow(t *testing.T) {
	path := filepath.Join(t.TempDir(), "d.jsonl")
	l, err := debuglog.New(path)
	if err != nil {
		t.Fatalf("debuglog.New: %v", err)
	}
	ad := agent.Adapter{Key: "stub", DisplayName: "Stub", Bin: "true",
		Args: func(p string) []string { return nil }}
	m := NewModel(ad, stats.Stats{}, t.TempDir()+"/stats.json", true, ".", l)

	m2, _ := m.Update(startHandMsg{})
	m = m2.(Model)
	m2, _ = m.Update(key("f")) // human folds hand 1
	m = m2.(Model)
	l.Close()

	byEvent := map[string][]map[string]any{}
	for _, e := range readLogEvents(t, path) {
		name := e["event"].(string)
		byEvent[name] = append(byEvent[name], e)
	}

	if n := len(byEvent["hand_start"]); n != 1 {
		t.Fatalf("hand_start count = %d, want 1", n)
	}
	hs := byEvent["hand_start"][0]
	if hs["hand"] != float64(1) || hs["sb"] != float64(10) || hs["bb"] != float64(20) {
		t.Errorf("hand_start = %v", hs)
	}
	for _, k := range []string{"hole_seat0", "hole_seat1", "stacks", "human_seat"} {
		if _, ok := hs[k]; !ok {
			t.Errorf("hand_start missing %q", k)
		}
	}

	if n := len(byEvent["human_key"]); n != 1 {
		t.Fatalf("human_key count = %d, want 1", n)
	}
	hk := byEvent["human_key"][0]
	if hk["key"] != "f" || hk["phase"] != "human_turn" {
		t.Errorf("human_key = %v", hk)
	}

	if n := len(byEvent["apply"]); n != 1 {
		t.Fatalf("apply count = %d, want 1", n)
	}
	ap := byEvent["apply"][0]
	if ap["action"] != "fold" || ap["actor_seat"] != float64(0) {
		t.Errorf("apply = %v", ap)
	}
	for _, k := range []string{"street", "pot", "stacks", "board", "current_bet", "committed"} {
		if _, ok := ap[k]; !ok {
			t.Errorf("apply missing %q", k)
		}
	}
	if _, ok := ap["error"]; ok {
		t.Error("apply has error field on legal action")
	}

	if n := len(byEvent["hand_end"]); n != 1 {
		t.Fatalf("hand_end count = %d, want 1", n)
	}
	he := byEvent["hand_end"][0]
	if he["winner_seat"] != float64(1) || he["showdown"] != false {
		t.Errorf("hand_end = %v", he)
	}
}

func TestAgentDecisionLogged(t *testing.T) {
	path := filepath.Join(t.TempDir(), "d.jsonl")
	l, err := debuglog.New(path)
	if err != nil {
		t.Fatalf("debuglog.New: %v", err)
	}
	ad := agent.Adapter{Key: "stub", DisplayName: "Stub", Bin: "true",
		Args: func(p string) []string { return nil }}
	m := NewModel(ad, stats.Stats{}, t.TempDir()+"/stats.json", true, ".", l)

	m2, _ := m.Update(startHandMsg{})
	m = m2.(Model)
	m2, _ = m.Update(key("c")) // human limps; agent's turn
	m = m2.(Model)
	m2, _ = m.Update(decisionMsg{act: poker.Action{Type: poker.Check}, say: "hm", fallback: true})
	m = m2.(Model)
	l.Close()

	var dec map[string]any
	for _, e := range readLogEvents(t, path) {
		if e["event"] == "agent_decision" {
			dec = e
		}
	}
	if dec == nil {
		t.Fatal("no agent_decision event logged")
	}
	if dec["action"] != "check" || dec["say"] != "hm" || dec["fallback"] != true {
		t.Errorf("agent_decision = %v", dec)
	}
}

func TestFallbackNoticeVisibleInQuietMode(t *testing.T) {
	ad := agent.Adapter{Key: "stub", DisplayName: "Stub", Bin: "true",
		Args: func(p string) []string { return nil }}
	m := NewModel(ad, stats.Stats{}, t.TempDir()+"/stats.json", true, ".", nil)
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

func typeString(t *testing.T, m Model, s string) Model {
	t.Helper()
	for _, r := range s {
		m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = m2.(Model)
	}
	return m
}

func TestTalkEchoRendersAndClearsNextHand(t *testing.T) {
	m := testModel(t)
	m2, _ := m.Update(startHandMsg{})
	m = m2.(Model)
	m2, _ = m.Update(key("t"))
	m = m2.(Model)
	if m.phase != phaseTalkInput {
		t.Fatalf("phase = %v, want phaseTalkInput", m.phase)
	}
	m = typeString(t, m, "read em and weep")
	m2, _ = m.Update(key("enter"))
	m = m2.(Model)
	if m.phase != phaseHumanTurn {
		t.Fatalf("phase after enter = %v, want phaseHumanTurn", m.phase)
	}
	if !strings.Contains(stripANSI(m.View()), `you: "read em and weep"`) {
		t.Error("echo line missing from view")
	}
	if !strings.Contains(m.digest.HandTalk(), "HUMAN: read em and weep") {
		t.Error("talk missing from digest")
	}
	// fold ends the hand; enter starts the next: echo must clear
	m2, _ = m.Update(key("f"))
	m = m2.(Model)
	m2, _ = m.Update(key("enter"))
	m = m2.(Model)
	if strings.Contains(stripANSI(m.View()), "you:") {
		t.Error("echo should clear at next hand start")
	}
}

func TestTalkEchoHiddenInQuietMode(t *testing.T) {
	ad := agent.Adapter{Key: "stub", DisplayName: "Stub", Bin: "true",
		Args: func(p string) []string { return nil }}
	m := NewModel(ad, stats.Stats{}, t.TempDir()+"/stats.json", true, ".", nil)
	m2, _ := m.Update(startHandMsg{})
	m = m2.(Model)
	m2, _ = m.Update(key("t"))
	m = m2.(Model)
	m = typeString(t, m, "silence")
	m2, _ = m.Update(key("enter"))
	m = m2.(Model)
	if strings.Contains(stripANSI(m.View()), "you:") {
		t.Error("echo must be hidden in quiet mode")
	}
}

func TestTalkAtHandEndReturnsToHandEnd(t *testing.T) {
	m := testModel(t)
	m2, _ := m.Update(startHandMsg{})
	m = m2.(Model)
	m2, _ = m.Update(key("f")) // fold -> phaseHandEnd
	m = m2.(Model)
	if m.phase != phaseHandEnd {
		t.Fatalf("phase = %v, want phaseHandEnd", m.phase)
	}
	m2, _ = m.Update(key("t"))
	m = m2.(Model)
	if m.phase != phaseTalkInput {
		t.Fatalf("t at hand end: phase = %v, want phaseTalkInput", m.phase)
	}
	m = typeString(t, m, "lucky fold")
	m2, _ = m.Update(key("enter"))
	m = m2.(Model)
	if m.phase != phaseHandEnd {
		t.Fatalf("phase after enter = %v, want phaseHandEnd", m.phase)
	}
	if !strings.Contains(m.digest.HandTalk(), "HUMAN: lucky fold") {
		t.Error("hand-end talk missing from digest")
	}
}

func TestTalkEscRestoresOriginPhase(t *testing.T) {
	m := testModel(t)
	m2, _ := m.Update(startHandMsg{})
	m = m2.(Model)
	m2, _ = m.Update(key("f")) // -> phaseHandEnd
	m = m2.(Model)
	m2, _ = m.Update(key("t"))
	m = m2.(Model)
	m2, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = m2.(Model)
	if m.phase != phaseHandEnd {
		t.Fatalf("esc from hand-end talk: phase = %v, want phaseHandEnd", m.phase)
	}
}

func TestTalkDuringAgentTurnSurvivesDecision(t *testing.T) {
	m := testModel(t)
	m2, _ := m.Update(startHandMsg{})
	m = m2.(Model)
	m2, _ = m.Update(key("c")) // human limps -> agent (BB) turn
	m = m2.(Model)
	if m.phase != phaseAgentTurn {
		t.Fatalf("phase = %v, want phaseAgentTurn", m.phase)
	}
	m2, _ = m.Update(key("t"))
	m = m2.(Model)
	if m.phase != phaseTalkInput {
		t.Fatalf("t during agent turn: phase = %v, want phaseTalkInput", m.phase)
	}
	m = typeString(t, m, "hurry up")
	// agent's decision lands mid-typing: BB checks, flop comes, agent acts first
	m2, _ = m.Update(decisionMsg{act: poker.Action{Type: poker.Check}})
	m = m2.(Model)
	if m.phase != phaseTalkInput {
		t.Fatalf("decision clobbered talk input: phase = %v", m.phase)
	}
	if m.hand.Street != poker.Flop {
		t.Fatalf("street = %v, want Flop (decision must still apply)", m.hand.Street)
	}
	m2, _ = m.Update(key("enter"))
	m = m2.(Model)
	if m.phase != phaseAgentTurn {
		t.Fatalf("phase after enter = %v, want phaseAgentTurn (post-decision phase)", m.phase)
	}
	if !strings.Contains(m.digest.HandTalk(), "HUMAN: hurry up") {
		t.Error("talk missing from digest")
	}
}

func TestTalkSurvivesShowdownRunout(t *testing.T) {
	m := testModel(t)
	m2, _ := m.Update(startHandMsg{})
	m = m2.(Model)
	m2, _ = m.Update(key("a")) // human shoves -> agent turn
	m = m2.(Model)
	if m.phase != phaseAgentTurn {
		t.Fatalf("phase = %v, want phaseAgentTurn", m.phase)
	}
	m2, _ = m.Update(key("t"))
	m = m2.(Model)
	m = typeString(t, m, "gg")
	// agent calls the shove mid-typing -> showdown -> runout
	m2, _ = m.Update(decisionMsg{act: poker.Action{Type: poker.Call}})
	m = m2.(Model)
	if m.phase != phaseTalkInput {
		t.Fatalf("phase = %v, want phaseTalkInput preserved over runout", m.phase)
	}
	if m.talkReturn != phaseRunout {
		t.Fatalf("talkReturn = %v, want phaseRunout", m.talkReturn)
	}
	// ticks must advance the reveal behind the input box
	for i := 0; i < len(m.hand.Board); i++ {
		var cmd tea.Cmd
		m2, cmd = m.Update(runoutTickMsg{})
		m = m2.(Model)
		if i < len(m.hand.Board)-1 && cmd == nil {
			t.Fatalf("tick %d: reveal chain broke (nil cmd) while typing", i)
		}
	}
	if m.revealed != len(m.hand.Board) {
		t.Fatalf("revealed = %d, want %d (ticks must advance while typing)", m.revealed, len(m.hand.Board))
	}
	if m.phase != phaseTalkInput || m.talkReturn != phaseHandEnd {
		t.Fatalf("after full reveal: phase = %v talkReturn = %v, want phaseTalkInput/phaseHandEnd", m.phase, m.talkReturn)
	}
	m2, _ = m.Update(key("enter"))
	m = m2.(Model)
	if m.phase != phaseHandEnd {
		t.Fatalf("phase after enter = %v, want phaseHandEnd", m.phase)
	}
}

func TestViewHidesRunoutWhileTyping(t *testing.T) {
	m := testModel(t)
	m2, _ := m.Update(startHandMsg{})
	m = m2.(Model)
	m2, _ = m.Update(key("a")) // human shoves
	m = m2.(Model)
	m2, _ = m.Update(key("t"))
	m = m2.(Model)
	m = typeString(t, m, "gg")
	m2, _ = m.Update(decisionMsg{act: poker.Action{Type: poker.Call}}) // showdown mid-typing
	m = m2.(Model)
	if m.phase != phaseTalkInput || m.talkReturn != phaseRunout {
		t.Fatalf("setup: phase %v talkReturn %v", m.phase, m.talkReturn)
	}
	view := stripANSI(m.View())
	// cardMidLine extracts the middle line of a face-up rendered card, e.g. "│A♠│".
	// That string appears in the view iff the card is rendered face-up.
	cardMidLine := func(c poker.Card) string {
		lines := strings.Split(stripANSI(RenderCard(c, true)), "\n")
		return lines[1]
	}
	// Build set of mid-lines for the human's hole cards (legitimately shown face-up).
	humanVisible := map[string]bool{}
	for _, c := range m.hand.Hole[m.humanSeat()] {
		humanVisible[cardMidLine(c)] = true
	}
	for _, c := range m.hand.Hole[m.agentSeat()] {
		ml := cardMidLine(c)
		if humanVisible[ml] {
			continue // ambiguous — same render as a human card shown face-up
		}
		if strings.Contains(view, ml) {
			t.Errorf("agent hole card %v visible during typing-over-runout (mid-line %q in view)", c, ml)
		}
	}
	// board must respect m.revealed (0 right after decision): no board card visible yet
	for _, c := range m.hand.Board {
		ml := cardMidLine(c)
		if humanVisible[ml] {
			continue
		}
		if strings.Contains(view, ml) {
			t.Errorf("board card %v visible before reveal tick (mid-line %q in view)", c, ml)
		}
	}
}

func TestTalkSurvivesAgentFoldToHandEnd(t *testing.T) {
	m := testModel(t)
	m2, _ := m.Update(startHandMsg{})
	m = m2.(Model)
	m2, _ = m.Update(key("a")) // human shoves -> agent turn
	m = m2.(Model)
	m2, _ = m.Update(key("t"))
	m = m2.(Model)
	m = typeString(t, m, "scared?")
	m2, _ = m.Update(decisionMsg{act: poker.Action{Type: poker.Fold}})
	m = m2.(Model)
	if m.phase != phaseTalkInput || m.talkReturn != phaseHandEnd {
		t.Fatalf("phase %v talkReturn %v, want phaseTalkInput/phaseHandEnd", m.phase, m.talkReturn)
	}
	m2, _ = m.Update(key("enter"))
	m = m2.(Model)
	if m.phase != phaseHandEnd {
		t.Fatalf("phase after enter = %v, want phaseHandEnd", m.phase)
	}
}
