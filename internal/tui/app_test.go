package tui

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/wuhaohan772/showdown/internal/agent"
	"github.com/wuhaohan772/showdown/internal/debuglog"
	"github.com/wuhaohan772/showdown/internal/poker"
	"github.com/wuhaohan772/showdown/internal/stats"
)

func testModel(t *testing.T) Model {
	t.Helper()
	ad := agent.Adapter{Key: "stub", DisplayName: "Stub", Bin: "true",
		Args: func(m, p string) []string { return nil }}
	return NewModel(ad, stats.Stats{}, t.TempDir()+"/stats.json", false, ".", "test-persona", 1500, 10, 0, nil)
}

func TestNewModelUsesCustomStackAndBlind(t *testing.T) {
	ad := agent.Adapter{Key: "stub", DisplayName: "Stub", Bin: "true",
		Args: func(m, p string) []string { return nil }}
	m := NewModel(ad, stats.Stats{}, t.TempDir()+"/stats.json", true, ".", "", 3000, 25, 0, nil)
	if m.match.Stacks != [2]int{3000, 3000} {
		t.Errorf("Stacks = %v, want [3000 3000]", m.match.Stacks)
	}
	sb, bb := m.match.Blinds()
	if sb != 25 || bb != 50 {
		t.Errorf("Blinds = %d/%d, want 25/50", sb, bb)
	}
}

func TestNewModelUsesCustomHandLimit(t *testing.T) {
	ad := agent.Adapter{Key: "stub", DisplayName: "Stub", Bin: "true",
		Args: func(m, p string) []string { return nil }}
	m := NewModel(ad, stats.Stats{}, t.TempDir()+"/stats.json", true, ".", "", 1500, 10, 5, nil)
	m.match.HandNum = 6
	m.match.Stacks = [2]int{1800, 1200}
	if !m.match.Over() {
		t.Error("match should be over: hand limit 5 reached with unequal stacks")
	}
}

func TestMatchOverOutcomeStatesTrueEndingReason(t *testing.T) {
	cases := []struct {
		name             string
		endedByBust      bool
		agentWon         bool
		wantSubstring    string
		wantNotSubstring string
	}{
		{"agent wins by bust", true, true, "Your human is busted", "Hand limit"},
		{"agent loses by bust", true, false, "They took every chip", "Hand limit"},
		{"agent wins by hand limit", false, true, "Hand limit hit", "busted"},
		{"agent loses by hand limit", false, false, "Hand limit hit", "busted"},
	}
	for _, c := range cases {
		got := matchOverOutcome(c.endedByBust, c.agentWon)
		if !strings.Contains(got, c.wantSubstring) {
			t.Errorf("%s: outcome = %q, want substring %q", c.name, got, c.wantSubstring)
		}
		if strings.Contains(got, c.wantNotSubstring) {
			t.Errorf("%s: outcome = %q, should not mention %q", c.name, got, c.wantNotSubstring)
		}
	}
}

func key(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func TestHandStartsWithHumanOrAgentTurn(t *testing.T) {
	t.Setenv("SHOWDOWN_REDUCE_MOTION", "1")
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

// Per-hand reactions were removed (each was a full CLI spawn); the only
// reaction call now fires once, at match end.
func TestNoReactionSpawnAtHandEnd(t *testing.T) {
	t.Setenv("SHOWDOWN_REDUCE_MOTION", "1")
	m := testModel(t) // quiet=false: the per-hand reaction used to fire here
	m2, _ := m.Update(startHandMsg{})
	m = m2.(Model)
	m2, cmd := m.Update(key("f")) // human folds, hand ends without showdown
	m = m2.(Model)
	if m.phase != phaseHandEnd {
		t.Fatalf("phase = %v, want phaseHandEnd", m.phase)
	}
	if cmd != nil {
		t.Error("fold-end hand should not spawn any command (reaction call removed)")
	}
}

func TestMatchEndReactionFired(t *testing.T) {
	m := testModel(t)
	m2, _ := m.Update(startHandMsg{})
	m = m2.(Model)
	hs, as := m.humanSeat(), m.agentSeat()
	m.hand.Seats[hs].Stack = 3000
	m.hand.Seats[as].Stack = 0
	if cmd := m.settleAndNext(); cmd == nil {
		t.Error("match over should fire one reaction command")
	}

	q := testModel(t)
	q.quiet = true
	m2, _ = q.Update(startHandMsg{})
	q = m2.(Model)
	hs, as = q.humanSeat(), q.agentSeat()
	q.hand.Seats[hs].Stack = 3000
	q.hand.Seats[as].Stack = 0
	if cmd := q.settleAndNext(); cmd != nil {
		t.Error("quiet mode must not fire a match-end reaction")
	}
}

func TestHumanFoldEndsHand(t *testing.T) {
	t.Setenv("SHOWDOWN_REDUCE_MOTION", "1")
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
	t.Setenv("SHOWDOWN_REDUCE_MOTION", "1")
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
	last := m.feed[len(m.feed)-1]
	if last.who != "Stub" || last.text != "hm" {
		t.Errorf("feed tail = %+v, want Stub: hm", last)
	}
}

func TestRaiseInputFlow(t *testing.T) {
	t.Setenv("SHOWDOWN_REDUCE_MOTION", "1")
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
	t.Setenv("SHOWDOWN_REDUCE_MOTION", "1")
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

func TestAgentHandSummarySplitPot(t *testing.T) {
	r := &poker.Result{Split: true, Pot: 40, Desc: "chopped, both had two pair"}
	got := agentHandSummary(3, r, false)
	want := "Hand 3: split pot (40) — chopped, both had two pair."
	if got != want {
		t.Errorf("agentHandSummary = %q, want %q", got, want)
	}
}

// The digest is read by the agent, so "you" must always mean the agent and
// the folder must be named explicitly — a bare "opponent folded" was
// ambiguous and made the agent misattribute actions in its trash talk.
func TestAgentHandSummaryPerspective(t *testing.T) {
	r := &poker.Result{Showdown: true, Pot: 80, Desc: "a pair of kings"}
	if got := agentHandSummary(5, r, true); got != "Hand 5: you won 80 (a pair of kings)." {
		t.Errorf("agent showdown win = %q", got)
	}
	r2 := &poker.Result{Showdown: false, Pot: 20}
	if got := agentHandSummary(6, r2, false); got != "Hand 6: the human won 20 (you folded)." {
		t.Errorf("human fold win = %q", got)
	}
	if got := agentHandSummary(7, r2, true); got != "Hand 7: you won 20 (the human folded)." {
		t.Errorf("agent fold win = %q", got)
	}
}

// TestMatchOverShowsCorrectStacks guards against a regression where the
// match-over screen rendered stacks/hole cards using the seat mapping for
// the *next* hand (humanSeat()/agentSeat() flip once NextHand advances
// HandNum), even though it was still displaying the just-finished hand.
// That bug showed "YOU stack: 0" even when the human won the match.
func TestMatchOverShowsCorrectStacks(t *testing.T) {
	t.Setenv("SHOWDOWN_REDUCE_MOTION", "1")
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
		Args: func(m, p string) []string { return nil }}
	NewModel(ad, stats.Stats{}, t.TempDir()+"/stats.json", true, ".", "", 1500, 10, 0, l)
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
	t.Setenv("SHOWDOWN_REDUCE_MOTION", "1")
	path := filepath.Join(t.TempDir(), "d.jsonl")
	l, err := debuglog.New(path)
	if err != nil {
		t.Fatalf("debuglog.New: %v", err)
	}
	ad := agent.Adapter{Key: "stub", DisplayName: "Stub", Bin: "true",
		Args: func(m, p string) []string { return nil }}
	m := NewModel(ad, stats.Stats{}, t.TempDir()+"/stats.json", true, ".", "", 1500, 10, 0, l)

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
		Args: func(m, p string) []string { return nil }}
	m := NewModel(ad, stats.Stats{}, t.TempDir()+"/stats.json", true, ".", "", 1500, 10, 0, l)

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
	t.Setenv("SHOWDOWN_REDUCE_MOTION", "1")
	ad := agent.Adapter{Key: "stub", DisplayName: "Stub", Bin: "true",
		Args: func(m, p string) []string { return nil }}
	m := NewModel(ad, stats.Stats{}, t.TempDir()+"/stats.json", true, ".", "", 1500, 10, 0, nil)
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

func TestTalkEchoRendersAndPersistsAcrossHands(t *testing.T) {
	t.Setenv("SHOWDOWN_REDUCE_MOTION", "1")
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
	if !strings.Contains(stripANSI(m.View()), "you: read em and weep") {
		t.Error("echo line missing from view")
	}
	if !strings.Contains(m.digest.HandTalk(), "HUMAN: read em and weep") {
		t.Error("talk missing from digest")
	}
	// fold ends the hand; enter starts the next: the feed is scrollback,
	// so the line persists (the ephemeral-echo clear is gone by design)
	m2, _ = m.Update(key("f"))
	m = m2.(Model)
	m2, _ = m.Update(key("enter"))
	m = m2.(Model)
	if !strings.Contains(stripANSI(m.View()), "you: read em and weep") {
		t.Error("feed line should persist into the next hand")
	}
	if !strings.Contains(stripANSI(m.View()), "─ hand 2 ") {
		t.Error("view missing hand-2 divider after next hand starts")
	}
}

func TestTalkEchoHiddenInQuietMode(t *testing.T) {
	ad := agent.Adapter{Key: "stub", DisplayName: "Stub", Bin: "true",
		Args: func(m, p string) []string { return nil }}
	m := NewModel(ad, stats.Stats{}, t.TempDir()+"/stats.json", true, ".", "", 1500, 10, 0, nil)
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
	t.Setenv("SHOWDOWN_REDUCE_MOTION", "1")
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
	t.Setenv("SHOWDOWN_REDUCE_MOTION", "1")
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
	t.Setenv("SHOWDOWN_REDUCE_MOTION", "1")
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
	t.Setenv("SHOWDOWN_REDUCE_MOTION", "1")
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
	t.Setenv("SHOWDOWN_REDUCE_MOTION", "1")
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
	t.Setenv("SHOWDOWN_REDUCE_MOTION", "1")
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

func TestSessionUsageAccumulates(t *testing.T) {
	m := testModel(t)
	m2, _ := m.Update(startHandMsg{})
	m = m2.(Model)
	m2, _ = m.Update(decisionMsg{act: poker.Action{Type: poker.Check},
		usage: &agent.Usage{InputTokens: 100, OutputTokens: 5, CostUSD: 0.02}})
	m = m2.(Model)
	m2, _ = m.Update(reactionMsg{say: "gg",
		usage: &agent.Usage{InputTokens: 50, OutputTokens: 3, CostUSD: 0.01}})
	m = m2.(Model)
	if m.sessionUsage.InputTokens != 150 || m.sessionUsage.CostUSD != 0.03 {
		t.Errorf("sessionUsage = %+v", m.sessionUsage)
	}
}

func TestKeyHintRows(t *testing.T) {
	t.Setenv("SHOWDOWN_REDUCE_MOTION", "1")
	m := testModel(t)
	m2, _ := m.Update(startHandMsg{})
	m = m2.(Model)
	m2, _ = m.Update(key("c")) // human limps -> agent turn
	m = m2.(Model)
	if m.phase != phaseAgentTurn {
		t.Fatalf("phase = %v, want phaseAgentTurn", m.phase)
	}
	if !strings.Contains(stripANSI(m.View()), "> (t)alk") {
		t.Error("agent-turn view missing talk hint row")
	}

	m = testModel(t)
	m2, _ = m.Update(startHandMsg{})
	m = m2.(Model)
	m2, _ = m.Update(key("f")) // fold -> hand end
	m = m2.(Model)
	if m.phase != phaseHandEnd {
		t.Fatalf("phase = %v, want phaseHandEnd", m.phase)
	}
	view := stripANSI(m.View())
	if !strings.Contains(view, "> (enter) next hand  (t)alk") {
		t.Error("hand-end view missing key hint row")
	}
	if strings.Contains(view, "enter for next hand") {
		t.Error("banner still contains redundant 'enter for next hand'")
	}
}

func TestLateReactionUsageReachesStats(t *testing.T) {
	m := testModel(t)
	m2, _ := m.Update(startHandMsg{})
	m = m2.(Model)
	m.sessionUsage = agent.Usage{InputTokens: 1000, OutputTokens: 200, CostUSD: 0.05}

	hs, as := m.humanSeat(), m.agentSeat()
	m.hand.Seats[hs].Stack = 3000
	m.hand.Seats[as].Stack = 0
	m.settleAndNext() // saves stats at match-over

	// reaction from the final hand lands late, after the save
	m2, _ = m.Update(reactionMsg{say: "gg", usage: &agent.Usage{InputTokens: 100, OutputTokens: 10, CostUSD: 0.01}})
	m = m2.(Model)

	saved, err := stats.Load(m.statsPath)
	if err != nil {
		t.Fatalf("load stats: %v", err)
	}
	r := saved["stub"]
	if r.TokensIn != 1100 || r.TokensOut != 210 {
		t.Errorf("late reaction usage not persisted: %+v", r)
	}
	if r.CostUSD < 0.059 || r.CostUSD > 0.061 {
		t.Errorf("CostUSD = %v, want ~0.06", r.CostUSD)
	}
}

func TestMatchOverSavesAndShowsUsage(t *testing.T) {
	m := testModel(t)
	m2, _ := m.Update(startHandMsg{})
	m = m2.(Model)
	m.sessionUsage = agent.Usage{InputTokens: 1000, OutputTokens: 200, CacheRead: 500, CostUSD: 0.09}

	hs, as := m.humanSeat(), m.agentSeat()
	m.hand.Seats[hs].Stack = 3000
	m.hand.Seats[as].Stack = 0
	m.settleAndNext()

	if m.phase != phaseMatchOver {
		t.Fatalf("phase = %v, want phaseMatchOver", m.phase)
	}
	view := stripANSI(m.View())
	if !strings.Contains(view, "tokens this match: 1500 in / 200 out · $0.09") {
		t.Errorf("view missing usage line, got:\n%s", view)
	}
	saved, err := stats.Load(m.statsPath)
	if err != nil {
		t.Fatalf("load stats: %v", err)
	}
	r := saved["stub"]
	if r.TokensIn != 1500 || r.TokensOut != 200 || r.CostUSD != 0.09 {
		t.Errorf("saved record = %+v", r)
	}
}

func TestDecisionPromptCarriesPersonality(t *testing.T) {
	m := testModel(t)
	m.personality = "PERSONA-MARKER"
	m2, _ := m.Update(startHandMsg{})
	m = m2.(Model)
	// force the agent-turn path regardless of button position
	data := agent.RequestData{
		AgentName:   m.opp.DisplayName,
		Personality: m.personality,
	}
	if !strings.Contains(agent.RenderPrompt(data), "PERSONA-MARKER") {
		t.Error("prompt built without personality")
	}
}

// The stub adapter has no SessionArgs, so the session path must stay
// completely inert: no session created, decisions still work (covered by
// existing tests), CloseSession safe.
func TestNoSessionForStatelessAdapter(t *testing.T) {
	m := testModel(t)
	m2, _ := m.Update(startHandMsg{})
	m = m2.(Model)
	if m.session != nil {
		t.Error("stateless adapter must not get a session")
	}
	m.CloseSession() // nil session: must not panic
}

// TestHelperFakeClaudeTUI is not a test: it is the fake claude CLI used by
// TUI session-path tests (standard Go helper-process pattern — same as
// TestHelperFakeClaude in session_test.go).  It reads JSONL user messages from
// stdin and emits a call-decision result event for each one.
func TestHelperFakeClaudeTUI(t *testing.T) {
	if os.Getenv("GO_FAKECLAUDE_TUI") != "1" {
		t.Skip("helper process")
	}
	sc := bufio.NewScanner(os.Stdin)
	enc := json.NewEncoder(os.Stdout)
	for sc.Scan() {
		_ = enc.Encode(map[string]any{
			"type": "result", "subtype": "success",
			// result is a JSON string whose value is a valid decision object.
			"result":         `{"action":"call"}`,
			"total_cost_usd": 0.01,
			"usage": map[string]any{
				"input_tokens": 1, "output_tokens": 1,
				"cache_read_input_tokens": 0, "cache_creation_input_tokens": 0,
			},
		})
	}
	os.Exit(0)
}

// sessionAdapter returns an Adapter whose session is backed by this test
// binary (helper-process pattern).  The stateless fallback uses a no-op run
// that exits immediately so it never blocks or pollutes the log.
func sessionAdapter(t *testing.T) agent.Adapter {
	t.Helper()
	return agent.Adapter{
		Key: "fake", DisplayName: "Fake", Bin: os.Args[0],
		// Stateless fallback: run with a regex that matches nothing — exits fast.
		Args:        func(m, p string) []string { return []string{"-test.run=^$"} },
		SessionArgs: func(m string) []string { return []string{"-test.run=TestHelperFakeClaudeTUI"} },
		Env:         []string{"GO_FAKECLAUDE_TUI=1"},
	}
}

// TestSessionPrimingThenDelta verifies that the first agent decision sends the
// full prompt (priming) while the second sends the compact delta: the session
// path must not re-send "=== YOUR TABLE PERSONA ===" on every turn.
//
// askAgentCmd is called directly (not through Update) to avoid the implicit
// session start that advance() would trigger on the agent's turn.
func TestSessionPrimingThenDelta(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "debug.jsonl")
	l, err := debuglog.New(logPath)
	if err != nil {
		t.Fatalf("debuglog.New: %v", err)
	}
	ad := sessionAdapter(t)
	m := NewModel(ad, stats.Stats{}, t.TempDir()+"/stats.json", true, ".", "PERSONA-MARKER", 1500, 10, 0, l)
	m2, _ := m.Update(startHandMsg{})
	m = m2.(Model)
	// After startHandMsg it is human turn (no askAgentCmd fired yet) so
	// m.session is nil — the first askAgentCmd call will start and prime it.
	if m.session != nil {
		t.Fatal("session must be nil before any agent decision")
	}

	// First decision: new session, priming turn — full prompt to the session.
	cmd1 := m.askAgentCmd()
	msg1 := cmd1()
	if _, ok := msg1.(decisionMsg); !ok {
		t.Fatalf("cmd1 returned %T, want decisionMsg", msg1)
	}
	if !m.session.Primed() {
		t.Error("session should be primed after first decision")
	}

	// Second decision: same session, delta turn — compact prompt, no persona.
	cmd2 := m.askAgentCmd()
	msg2 := cmd2()
	if _, ok := msg2.(decisionMsg); !ok {
		t.Fatalf("cmd2 returned %T, want decisionMsg", msg2)
	}

	l.Close()

	var agentCalls []map[string]any
	for _, e := range readLogEvents(t, logPath) {
		if e["event"] == "agent_call" {
			agentCalls = append(agentCalls, e)
		}
	}
	if len(agentCalls) < 2 {
		t.Fatalf("want at least 2 agent_call log entries, got %d (fake script may not have responded)", len(agentCalls))
	}
	first := agentCalls[0]["prompt"].(string)
	second := agentCalls[1]["prompt"].(string)
	if !strings.Contains(first, "=== YOUR TABLE PERSONA ===") {
		t.Error("first (priming) call must contain full prompt with persona section")
	}
	if strings.Contains(second, "=== YOUR TABLE PERSONA ===") {
		t.Error("second (delta) call must NOT contain persona section")
	}
}

// TestSessionRestartAfterDeath verifies that when a session is killed the next
// decision starts a fresh session and re-primes with the full prompt.
func TestSessionRestartAfterDeath(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "debug.jsonl")
	l, err := debuglog.New(logPath)
	if err != nil {
		t.Fatalf("debuglog.New: %v", err)
	}
	ad := sessionAdapter(t)
	m := NewModel(ad, stats.Stats{}, t.TempDir()+"/stats.json", true, ".", "PERSONA-MARKER", 1500, 10, 0, l)
	m2, _ := m.Update(startHandMsg{})
	m = m2.(Model)

	// First decision establishes the session (priming).
	cmd1 := m.askAgentCmd()
	_ = cmd1()
	if m.session == nil || !m.session.Primed() {
		t.Fatal("session must be alive and primed after first decision")
	}

	// Simulate session death (e.g. process crash or rate-limit kill).
	m.session.Close()
	if m.session.Alive() {
		t.Fatal("session should be dead after Close")
	}

	// Second decision: ensureSession must restart and re-prime with full prompt.
	cmd2 := m.askAgentCmd()
	_ = cmd2()

	l.Close()

	var sessionStarts []map[string]any
	var agentCalls []map[string]any
	for _, e := range readLogEvents(t, logPath) {
		switch e["event"] {
		case "agent_session_start":
			sessionStarts = append(sessionStarts, e)
		case "agent_call":
			agentCalls = append(agentCalls, e)
		}
	}
	// Find the restart event (there may be no earlier starts since we called
	// askAgentCmd directly without going through Update on a human turn).
	var restartSeen bool
	for _, e := range sessionStarts {
		if e["restart"] == true {
			restartSeen = true
		}
	}
	if !restartSeen {
		t.Errorf("want an agent_session_start with restart=true; got starts: %v", sessionStarts)
	}
	// After restart the priming turn re-sends the full prompt.
	if len(agentCalls) < 2 {
		t.Fatalf("want at least 2 agent_call events (one per decision), got %d", len(agentCalls))
	}
	// Find the post-restart agent_call: it should be the last one.
	lastPrompt := agentCalls[len(agentCalls)-1]["prompt"].(string)
	if !strings.Contains(lastPrompt, "=== YOUR TABLE PERSONA ===") {
		t.Error("post-restart decision must re-prime with full prompt (persona section required)")
	}
}

// ── chat panel + match-end reaction (spec 2026-07-10) ──

func TestChatPanelRendersOnWideTerminal(t *testing.T) {
	m := testModel(t)
	m2, _ := m.Update(startHandMsg{})
	m = m2.(Model)
	m2, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = m2.(Model)
	m.feed = append(m.feed,
		chatLine{who: "Stub", text: "a very long taunt that has no choice but to wrap onto a continuation line"},
		chatLine{who: "you", text: "bring it"})
	view := stripANSI(m.View())
	if !strings.Contains(view, "talk") {
		t.Error("panel header missing")
	}
	if !strings.Contains(view, "Stub: a very long taunt") {
		t.Error("speaker-prefixed line missing from panel")
	}
	if !strings.Contains(view, "you: bring it") {
		t.Error("human line missing from panel")
	}
	if !strings.Contains(view, "─ hand 1 ") {
		t.Error("hand divider missing from panel")
	}
	// full-height separator: every row of the joined view carries the bar,
	// which also proves the two columns were padded to equal height
	for i, ln := range strings.Split(strings.TrimRight(view, "\n"), "\n") {
		if !strings.Contains(ln, "│") {
			t.Errorf("row %d missing separator bar: %q", i, ln)
		}
	}
}

func TestChatLogFallbackOnNarrowOrUnknownWidth(t *testing.T) {
	t.Setenv("SHOWDOWN_REDUCE_MOTION", "1")
	m := testModel(t) // width == 0: tests, pipes
	m2, _ := m.Update(startHandMsg{})
	m = m2.(Model)
	m.feed = append(m.feed, chatLine{who: "Stub", text: "small table talk"})
	view := stripANSI(m.View())
	if !strings.Contains(view, "Stub: small table talk") {
		t.Error("bottom log missing feed line")
	}
	logAt := strings.Index(view, "Stub: small table talk")
	barAt := strings.LastIndex(view, "> ")
	if barAt < logAt {
		t.Error("bottom log must render above the action bar")
	}
	if strings.Contains(view, "talk\n") && strings.Contains(view, " │ ") {
		t.Error("narrow view must not render the side panel")
	}
}

func TestQuietModeHasNoPanelOrLog(t *testing.T) {
	ad := agent.Adapter{Key: "stub", DisplayName: "Stub", Bin: "true",
		Args: func(m, p string) []string { return nil }}
	m := NewModel(ad, stats.Stats{}, t.TempDir()+"/stats.json", true, ".", "", 1500, 10, 0, nil)
	m2, _ := m.Update(startHandMsg{})
	m = m2.(Model)
	m2, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = m2.(Model)
	m.feed = append(m.feed, chatLine{who: "Stub", text: "SHOULD-NOT-SHOW"})
	if strings.Contains(stripANSI(m.View()), "SHOULD-NOT-SHOW") {
		t.Error("quiet mode must not render the feed anywhere")
	}
}

func TestMatchOverWaitsForReaction(t *testing.T) {
	m := testModel(t)
	m2, _ := m.Update(startHandMsg{})
	m = m2.(Model)
	hs, as := m.humanSeat(), m.agentSeat()
	m.hand.Seats[hs].Stack = 3000
	m.hand.Seats[as].Stack = 0
	if cmd := m.settleAndNext(); cmd == nil {
		t.Fatal("match over should fire the reaction command")
	}
	if !m.reactionPending {
		t.Fatal("reactionPending must be set while the reaction is in flight")
	}
	if !strings.Contains(stripANSI(m.View()), "is typing") {
		t.Error("view missing typing spinner while reaction pending")
	}

	// first q must not quit — it shows the skip hint
	m2, cmd := m.Update(key("q"))
	m = m2.(Model)
	if cmd != nil {
		t.Fatal("first q while reaction pending must not quit")
	}
	if !strings.Contains(stripANSI(m.View()), "waiting for parting shot") {
		t.Error("view missing skip hint after first q")
	}

	// reaction lands: feed gains the line, pending clears
	m2, _ = m.Update(reactionMsg{say: "well played, human"})
	m = m2.(Model)
	if m.reactionPending {
		t.Error("reactionPending must clear when the reaction arrives")
	}
	last := m.feed[len(m.feed)-1]
	if last.who != "Stub" || last.text != "well played, human" {
		t.Errorf("feed tail = %+v, want the reaction line", last)
	}

	// now q quits
	m2, cmd = m.Update(key("q"))
	if cmd == nil {
		t.Fatal("q after reaction must quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Errorf("cmd() = %T, want tea.QuitMsg", cmd())
	}
}

func TestMatchOverSecondQSkipsPendingReaction(t *testing.T) {
	m := testModel(t)
	m2, _ := m.Update(startHandMsg{})
	m = m2.(Model)
	hs, as := m.humanSeat(), m.agentSeat()
	m.hand.Seats[hs].Stack = 3000
	m.hand.Seats[as].Stack = 0
	m.settleAndNext()

	m2, cmd := m.Update(key("q")) // hint
	m = m2.(Model)
	if cmd != nil {
		t.Fatal("first q must not quit")
	}
	m2, cmd = m.Update(key("q")) // escape hatch
	if cmd == nil {
		t.Fatal("second q must quit even with the reaction still pending")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Errorf("cmd() = %T, want tea.QuitMsg", cmd())
	}
}

func TestPendingHandResultsAccumulateAndCarrySummaries(t *testing.T) {
	t.Setenv("SHOWDOWN_REDUCE_MOTION", "1")
	m := testModel(t)
	m2, _ := m.Update(startHandMsg{})
	m = m2.(Model)
	m2, _ = m.Update(key("f")) // human folds, hand ends
	m = m2.(Model)
	if len(m.pendingResults) != 1 || !strings.Contains(m.pendingResults[0], "Hand 1:") {
		t.Errorf("pendingResults = %v, want the hand 1 summary", m.pendingResults)
	}
}

// ── motion pass: anim ticker, deal phase, reduce-motion (spec task 3) ──

// tick pumps n animation frames through Update.
func tick(t *testing.T, m Model, n int) Model {
	t.Helper()
	for i := 0; i < n; i++ {
		mm, _ := m.Update(animTickMsg{})
		m = mm.(Model)
	}
	return m
}

func TestDealAnimationGatesPhase(t *testing.T) {
	m := testModel(t)
	mm, _ := m.Update(startHandMsg{})
	m = mm.(Model)
	if m.phase != phaseDealing {
		t.Fatalf("phase after startHand = %v, want phaseDealing", m.phase)
	}
	// 4 cards x 4 frames each = 16 frames to land everything; one extra frame
	// runs the phase transition. Pump plenty.
	m = tick(t, m, 25)
	if m.phase == phaseDealing {
		t.Fatalf("deal never completed; phase still dealing after 25 frames")
	}
	if m.phase != phaseHumanTurn && m.phase != phaseAgentTurn {
		t.Fatalf("post-deal phase = %v, want a turn phase", m.phase)
	}
}

func TestReduceMotionSkipsDeal(t *testing.T) {
	t.Setenv("SHOWDOWN_REDUCE_MOTION", "1")
	m := testModel(t)
	mm, _ := m.Update(startHandMsg{})
	m = mm.(Model)
	if m.phase == phaseDealing {
		t.Fatal("reduce-motion must skip the dealing phase entirely")
	}
	if m.animRunning {
		t.Fatal("reduce-motion must never start the anim ticker")
	}
}

func TestRaiseErrorBannerSettles(t *testing.T) {
	m := testModel(t)
	mm, _ := m.Update(startHandMsg{})
	m = mm.(Model)
	m = tick(t, m, 80) // deal + tweens fully settled, ticker idle
	if m.animRunning {
		t.Fatal("precondition: ticker must be idle before the raise input")
	}
	m.phase = phaseRaiseInput
	m.input.SetValue("not-a-number")
	mm, cmd := m.confirmInput()
	m = mm.(Model)
	if m.bannerAge != 0 {
		t.Fatal("error banner must stamp bright")
	}
	if cmd == nil || !m.animRunning {
		t.Fatal("error banner must restart the anim ticker so it can settle")
	}
	m = tick(t, m, bannerBright+1)
	if m.bannerAge < bannerBright {
		t.Errorf("banner never settled: age %d", m.bannerAge)
	}
}

func TestPotTweenReachesTarget(t *testing.T) {
	m := testModel(t)
	mm, _ := m.Update(startHandMsg{})
	m = mm.(Model)
	if m.potShown != 0 {
		t.Fatalf("potShown starts at %d, want 0", m.potShown)
	}
	m = tick(t, m, 60) // deal + tween settle well within 60 frames
	if m.potShown != m.potTarget() {
		t.Errorf("potShown = %d after settling, want target %d", m.potShown, m.potTarget())
	}
	if m.potTarget() != m.hand.Pot {
		t.Errorf("in-hand pot target = %d, want live pot %d", m.potTarget(), m.hand.Pot)
	}
}

func TestAnimTickerStopsWhenIdle(t *testing.T) {
	m := testModel(t)
	mm, _ := m.Update(startHandMsg{})
	m = mm.(Model)
	m = tick(t, m, 80)
	if m.animating() {
		t.Fatalf("model still animating after 80 frames: %+v", m)
	}
	if m.animRunning {
		t.Error("animRunning must clear once nothing animates")
	}
}

func TestBoardGrowthStartsSlide(t *testing.T) {
	m := testModel(t)
	mm, _ := m.Update(startHandMsg{})
	m = mm.(Model)
	m = tick(t, m, 25) // let the hole deal finish
	// Drive the engine to the flop directly through the apply funnel:
	// call + check ends preflop regardless of which seat is which.
	if err := m.apply(poker.Action{Type: poker.Call}); err != nil {
		t.Fatalf("call: %v", err)
	}
	if err := m.apply(poker.Action{Type: poker.Check}); err != nil {
		t.Fatalf("check: %v", err)
	}
	if len(m.hand.Board) != 3 {
		t.Fatalf("expected flop on board, got %d cards", len(m.hand.Board))
	}
	if !m.boardDeal.active() || m.boardDeal.total != 3 || m.boardDeal.landed != 0 {
		t.Errorf("flop must start a 3-card slide from 0, got %+v", m.boardDeal)
	}
	if m.boardSeen != 3 {
		t.Errorf("boardSeen = %d, want 3", m.boardSeen)
	}
}

func TestBoardGrowthNoSlideWhenReduced(t *testing.T) {
	t.Setenv("SHOWDOWN_REDUCE_MOTION", "1")
	m := testModel(t)
	mm, _ := m.Update(startHandMsg{})
	m = mm.(Model)
	_ = m.apply(poker.Action{Type: poker.Call})
	_ = m.apply(poker.Action{Type: poker.Check})
	if m.boardDeal.active() {
		t.Error("reduce-motion must not start a board slide")
	}
}

func TestTypewriterRevealsAgentSay(t *testing.T) {
	m := testModel(t)
	mm, _ := m.Update(startHandMsg{})
	m = mm.(Model)
	m = tick(t, m, 25)
	say := "you fold AK preflop? bold strategy"
	mm, _ = m.Update(decisionMsg{act: poker.Action{Type: poker.Check}, say: say})
	m = mm.(Model)
	if m.typeIdx != len(m.feed)-1 || m.typeShown != 0 {
		t.Fatalf("agent say must start a typewriter at the new feed line: idx=%d shown=%d", m.typeIdx, m.typeShown)
	}
	joined := strings.Join(m.feedLines(60), "\n")
	if strings.Contains(joined, say) {
		t.Fatalf("full say visible before any ticks: %q", joined)
	}
	if !strings.Contains(joined, "▌") {
		t.Errorf("typing line must show the cursor: %q", joined)
	}
	m = tick(t, m, len([]rune(say))) // 2 runes/frame: plenty
	if m.typeIdx != -1 {
		t.Fatalf("typewriter never finished: idx=%d shown=%d", m.typeIdx, m.typeShown)
	}
	joined = strings.Join(m.feedLines(60), "\n")
	if !strings.Contains(joined, say) || strings.Contains(joined, "▌") {
		t.Errorf("finished line must be full text without cursor: %q", joined)
	}
}

func TestHumanSayIsInstant(t *testing.T) {
	m := testModel(t)
	mm, _ := m.Update(startHandMsg{})
	m = mm.(Model)
	m.feed = append(m.feed, chatLine{who: "you", text: "read it and weep"})
	if m.typeIdx != -1 {
		t.Error("human lines must not start a typewriter")
	}
	if joined := strings.Join(m.feedLines(60), "\n"); !strings.Contains(joined, "read it and weep") {
		t.Errorf("human line must be fully visible immediately: %q", joined)
	}
}

func TestSetBannerStampsBright(t *testing.T) {
	m := testModel(t)
	mm, _ := m.Update(startHandMsg{})
	m = mm.(Model)
	if m.bannerAge != 0 {
		t.Fatalf("startHand must stamp the banner (age 0), got %d", m.bannerAge)
	}
	m = tick(t, m, bannerBright+1)
	if m.bannerAge < bannerBright {
		t.Errorf("banner age must reach settle threshold, got %d", m.bannerAge)
	}
	if !bannerStyleFor(0).GetBold() {
		t.Error("fresh banner style must be bold")
	}
	if bannerStyleFor(bannerBright).GetBold() {
		t.Error("settled banner style must not be bold")
	}
}

func TestHeaderShowsModelTag(t *testing.T) {
	m := testModel(t)
	m.opp.Model = "sonnet"
	mm, _ := m.Update(startHandMsg{})
	m = mm.(Model)
	table, _ := m.renderTable()
	if !strings.Contains(table, m.opp.DisplayName+" · sonnet") {
		t.Errorf("header must carry the model tag, got:\n%s", table)
	}
}
