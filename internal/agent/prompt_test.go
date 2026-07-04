package agent

import (
	"math/rand"
	"strings"
	"testing"

	"github.com/haohanwu/showdown/internal/poker"
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
	d := NewDigest()
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

// Doctrine v2: behavioral say rules live in FORMAT RULES because recency
// wins — body-paragraph placement failed live on haiku (2026-07-04
// regression: hand-only table talk). The needling default must be the last
// behavioral rule, after the card-talk rule, so it wins recency.
func TestNeedlingRuleLastInFormatRules(t *testing.T) {
	out := RenderPrompt(RequestData{AgentName: "Stub"})
	needle := strings.Index(out, "Default \"say\" material is THEM, not the cards")
	cards := strings.Index(out, "Card talk is a weapon, not a habit")
	if needle == -1 {
		t.Fatal("format rules missing the personalized-needling default rule")
	}
	if cards == -1 {
		t.Fatal("format rules missing the card-talk rule")
	}
	if needle < cards {
		t.Error("needling rule must come after the card-talk rule (recency wins)")
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
