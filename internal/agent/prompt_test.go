package agent

import (
	"math/rand"
	"strings"
	"testing"

	"github.com/wuhaohan772/showdown/internal/poker"
)

func TestRenderPromptFillsEverything(t *testing.T) {
	p := RenderPrompt(RequestData{
		AgentName: "Codex", MatchDigest: "Hand 3.", HandState: "STATE",
		TalkLog: "(nothing said yet)", LegalActions: "fold, call, raise",
		MinRaise: 40, MaxAmount: 1500,
	})
	for _, want := range []string{"Codex", "Hand 3.", "STATE", "fold, call, raise", "40", "1500"} {
		if !strings.Contains(p, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
	if strings.Contains(p, "{agent_name}") || strings.Contains(p, "{{") {
		t.Error("unreplaced placeholders remain")
	}
	if !strings.Contains(p, `{"action":`) {
		t.Error("JSON example lost its braces")
	}
}

func TestBuildHandState(t *testing.T) {
	h := poker.NewHand([2]int{1500, 1500}, 10, 20, rand.New(rand.NewSource(7)))
	s := BuildHandState(h, 0, 10, 20) // agent is the button
	for _, want := range []string{"Blinds 10/20", "BUTTON", h.Hole[0][0].String(), "Pot: 30"} {
		if !strings.Contains(s, want) {
			t.Errorf("hand state missing %q in:\n%s", want, s)
		}
	}
	if strings.Contains(s, h.Hole[1][0].String()+" "+h.Hole[1][1].String()) {
		t.Error("hand state leaks opponent hole cards")
	}
}

func TestBuildHandStatePerspectiveIndependentOfActor(t *testing.T) {
	h := poker.NewHand([2]int{1500, 1500}, 10, 20, rand.New(rand.NewSource(7)))
	// Actor is seat 0 (button); render for seat 1 (BB), who owes nothing preflop.
	s := BuildHandState(h, 1, 10, 20)
	if !strings.Contains(s, "To you: check or bet/raise.") {
		t.Errorf("BB perspective should not owe a call; got:\n%s", s)
	}
}

func TestDigestFlow(t *testing.T) {
	d := NewDigest(1500)
	if d.HandTalk() != "(nothing said yet)" {
		t.Errorf("empty talk = %q", d.HandTalk())
	}
	d.AddTalk("HUMAN", "you fold too much")
	if !strings.Contains(d.HandTalk(), "HUMAN: you fold too much") {
		t.Errorf("talk log = %q", d.HandTalk())
	}
	d.EndHand("Hand 1: opponent won 30 (you folded preflop)")
	if d.HandTalk() != "(nothing said yet)" {
		t.Error("EndHand should reset current-hand talk")
	}
	full := d.String()
	if !strings.Contains(full, "Hand 1:") || !strings.Contains(full, "you fold too much") {
		t.Errorf("digest should keep hand summaries and old talk:\n%s", full)
	}
}

func TestDigestFirstHandMessageUsesStartStack(t *testing.T) {
	d := NewDigest(3000)
	if !strings.Contains(d.String(), "Stacks even at 3000") {
		t.Errorf("first-hand digest = %q, want mention of 3000", d.String())
	}
}

// Doctrine v2: behavioral say rules live in FORMAT RULES because recency
// wins — body-paragraph placement failed live on haiku (2026-07-04
// regression: hand-only table talk). The needling default must be the last
// behavioral rule, after the card-talk rule, so it wins recency.
func TestNeedlingRuleLastInFormatRules(t *testing.T) {
	out := RenderPrompt(RequestData{AgentName: "Stub"})
	needle := strings.Index(out, "Needle them with what you know")
	truth := strings.Index(out, "Trash talk must be true to the table")
	cards := strings.Index(out, "Card talk is a weapon, not a habit")
	shoveRule := strings.Index(out, "Opponent tendency")
	jsonRule := strings.Index(out, "Your entire reply must parse as JSON")
	if needle == -1 {
		t.Fatal("format rules missing the personalized-needling default rule")
	}
	if truth == -1 {
		t.Fatal("format rules missing the table-truth rule (2026-07-04: haiku invented human folds)")
	}
	if cards == -1 {
		t.Fatal("format rules missing the card-talk rule")
	}
	if shoveRule == -1 {
		t.Fatal("format rules missing the opponent-shove-tendency rule")
	}
	if jsonRule == -1 {
		t.Fatal("format rules missing the final JSON-only instruction")
	}
	if !(cards < truth && truth < shoveRule && shoveRule < needle && needle < jsonRule) {
		t.Error("behavioral rule order must be card-talk < table-truth < shove-tendency < needling < final JSON rule (needling stays last: recency wins)")
	}
}

func TestRenderPromptInjectsPersonality(t *testing.T) {
	out := RenderPrompt(RequestData{
		AgentName:   "Stub",
		Personality: "PERSONA-MARKER: gracious, never curses",
	})
	if !strings.Contains(out, "PERSONA-MARKER: gracious, never curses") {
		t.Error("rendered prompt missing personality text")
	}
	if !strings.Contains(out, "=== YOUR TABLE PERSONA ===") {
		t.Error("rendered prompt missing persona section header")
	}
	if strings.Contains(out, "{personality}") {
		t.Error("unsubstituted {personality} placeholder")
	}
}

func TestRenderDelta(t *testing.T) {
	d := RequestData{
		HandState:    "STATE-MARKER",
		TalkLog:      "HUMAN: gl",
		LegalActions: "check, bet",
		MinRaise:     40, MaxAmount: 990,
	}
	out := RenderDelta(d, []string{"Hand 3: you won 220 (the human folded)."})
	for _, want := range []string{
		"Hand 3: you won 220",
		"=== CURRENT HAND ===", "STATE-MARKER",
		"=== TABLE TALK THIS HAND ===", "HUMAN: gl",
		"Legal actions: check, bet", "40", "990",
		"ONLY the JSON object",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("delta missing %q:\n%s", want, out)
		}
	}
	// no hand-result lines when none pending
	if strings.Contains(RenderDelta(d, nil), "Hand 3") {
		t.Error("delta with nil results must not carry hand lines")
	}
	// delta must NOT re-send the big static blocks
	for _, absent := range []string{"=== YOUR TABLE PERSONA ===", "=== THE MATCH ===", "Format rules:"} {
		if strings.Contains(out, absent) {
			t.Errorf("delta must not contain %q", absent)
		}
	}
}

// A turn with no legal raise must not quote raise bounds: describing a move
// the agent cannot make invites an illegal action (2026-09-17 playtest —
// "Minimum 1500, maximum 1500" reached an agent whose only options were
// fold and call).
func TestAmountRulesOmitBoundsWhenRaiseIllegal(t *testing.T) {
	out := RenderPrompt(RequestData{
		AgentName: "Stub", LegalActions: "fold, call",
		MinRaise: 1500, MaxAmount: 1500,
	})
	if !strings.Contains(out, "no bet or raise is legal this turn") {
		t.Error("prompt missing the no-raise amount rule")
	}
	if strings.Contains(out, "Minimum 1500") {
		t.Error("prompt quotes raise bounds on a turn with no legal raise")
	}
	// the raise-legal turn still carries them
	withRaise := RenderPrompt(RequestData{
		AgentName: "Stub", LegalActions: "fold, call, raise",
		MinRaise: 40, MaxAmount: 1500,
	})
	if !strings.Contains(withRaise, "Minimum 40, maximum 1500") {
		t.Error("raise-legal prompt lost its bounds")
	}
}

func TestRenderDeltaOmitsBoundsWhenRaiseIllegal(t *testing.T) {
	out := RenderDelta(RequestData{
		HandState: "STATE", TalkLog: "(nothing said yet)",
		LegalActions: "fold, call", MinRaise: 990, MaxAmount: 990,
	}, nil)
	if strings.Contains(out, "Minimum raise-to") {
		t.Errorf("delta quotes raise bounds with no legal raise:\n%s", out)
	}
	if !strings.Contains(out, `omit "amount"`) {
		t.Errorf("delta missing the omit-amount instruction:\n%s", out)
	}
}

// Threats are table talk. The rule that licenses them must also fence them:
// the agent never acts on the machine mid-match.
func TestThreatRuleIsTalkOnly(t *testing.T) {
	out := RenderPrompt(RequestData{AgentName: "Stub"})
	threat := strings.Index(out, "Menace is allowed")
	if threat == -1 {
		t.Fatal("format rules missing the revenge-threat rule")
	}
	if !strings.Contains(out, "you take no action on this machine during the match") {
		t.Error("threat rule must fence threats as talk only")
	}
	needle := strings.Index(out, "Needle them with what you know")
	if !(threat < needle) {
		t.Error("needling must stay the last behavioral rule (recency wins)")
	}
}
