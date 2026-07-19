package agent

import (
	"context"
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/wuhaohan772/showdown/internal/poker"
)

func TestExtractDecision(t *testing.T) {
	cases := []struct {
		raw    string
		action string
		amount int
	}{
		{`{"action": "raise", "amount": 60, "say": "hi"}`, "raise", 60},
		{"Sure! Here's my move:\n```json\n{\"action\": \"fold\"}\n```\nGood luck!", "fold", 0},
		{`{"thinking": "hmm"} {"action": "call"}`, "call", 0},
		{`{"action": "bet", "amount": 300, "say": "nested {braces} in talk"}`, "bet", 300},
	}
	for _, c := range cases {
		d, err := ExtractDecision(c.raw)
		if err != nil {
			t.Errorf("ExtractDecision(%q): %v", c.raw, err)
			continue
		}
		if d.Action != c.action || d.Amount != c.amount {
			t.Errorf("got %+v, want %s/%d", d, c.action, c.amount)
		}
	}
	if _, err := ExtractDecision("no json here at all"); err == nil {
		t.Error("want error for JSON-free reply")
	}
}

func legalFCR() []poker.ActionType { return []poker.ActionType{poker.Fold, poker.Call, poker.Raise} }

func TestValidateDecision(t *testing.T) {
	if _, err := ValidateDecision(Decision{Action: "check"}, legalFCR(), 40, 1500); err == nil {
		t.Error("illegal action should fail")
	}
	if _, err := ValidateDecision(Decision{Action: "raise", Amount: 30}, legalFCR(), 40, 1500); err == nil {
		t.Error("below-min raise should fail")
	}
	a, err := ValidateDecision(Decision{Action: "raise", Amount: 1500}, legalFCR(), 40, 1500)
	if err != nil || a.Type != poker.Raise || a.To != 1500 {
		t.Errorf("all-in raise: %+v, %v", a, err)
	}
}

func TestGetDecisionRetryThenFallback(t *testing.T) {
	calls := 0
	garbage := func(ctx context.Context, prompt string) (Response, error) {
		calls++
		return Response{Text: "I love poker!!!"}, nil
	}
	a, _, fb, _ := GetDecision(context.Background(), garbage, RequestData{}, legalFCR())
	if !fb || a.Type != poker.Fold {
		t.Errorf("garbage twice → fallback fold; got %+v fb=%v", a, fb)
	}
	if calls != 2 {
		t.Errorf("want exactly 2 calls (one retry), got %d", calls)
	}
}

func TestGetDecisionRetrySucceeds(t *testing.T) {
	calls := 0
	flaky := func(ctx context.Context, prompt string) (Response, error) {
		calls++
		if calls == 1 {
			return Response{Text: "hmm let me think"}, nil
		}
		return Response{Text: `{"action": "call", "say": "fine"}`}, nil
	}
	a, say, fb, _ := GetDecision(context.Background(), flaky, RequestData{}, legalFCR())
	if fb || a.Type != poker.Call || say != "fine" {
		t.Errorf("got %+v say=%q fb=%v", a, say, fb)
	}
}

func TestGetDecisionErrorNoRetry(t *testing.T) {
	calls := 0
	dead := func(ctx context.Context, prompt string) (Response, error) {
		calls++
		return Response{}, errors.New("timeout")
	}
	a, _, fb, _ := GetDecision(context.Background(), dead, RequestData{}, legalFCR())
	if !fb || a.Type != poker.Fold || calls != 1 {
		t.Errorf("exec error → immediate fallback, 1 call; got %+v fb=%v calls=%d", a, fb, calls)
	}
}

func TestFallbackPrefersCheck(t *testing.T) {
	a := FallbackAction([]poker.ActionType{poker.Check, poker.Bet})
	if a.Type != poker.Check {
		t.Errorf("got %v, want check", a.Type)
	}
}

func TestGetReactionRuneSafeTruncation(t *testing.T) {
	long := strings.Repeat("a", 118) + "——中文" // multi-byte runes straddling the cap
	ask := func(ctx context.Context, prompt string) (Response, error) { return Response{Text: long}, nil }
	got, _ := GetReaction(context.Background(), ask, "X", "", "d", "s")
	if !utf8.ValidString(got) {
		t.Errorf("truncated reaction is invalid UTF-8: %q", got)
	}
	if utf8.RuneCountInString(got) > 120 {
		t.Errorf("rune count %d > 120", utf8.RuneCountInString(got))
	}
}

func TestGetDecisionSumsUsageAcrossRetry(t *testing.T) {
	calls := 0
	flaky := func(ctx context.Context, prompt string) (Response, error) {
		calls++
		u := &Usage{InputTokens: 100, OutputTokens: 10, CostUSD: 0.01}
		if calls == 1 {
			return Response{Text: "hmm let me think", Usage: u}, nil
		}
		return Response{Text: `{"action": "call"}`, Usage: u}, nil
	}
	_, _, _, used := GetDecision(context.Background(), flaky, RequestData{}, legalFCR())
	if used == nil || used.InputTokens != 200 || used.OutputTokens != 20 {
		t.Errorf("used = %+v, want summed over 2 calls", used)
	}
}

func TestGetDecisionNilUsageStaysNil(t *testing.T) {
	plain := func(ctx context.Context, prompt string) (Response, error) {
		return Response{Text: `{"action": "call"}`}, nil
	}
	_, _, _, used := GetDecision(context.Background(), plain, RequestData{}, legalFCR())
	if used != nil {
		t.Errorf("used = %+v, want nil for usage-less adapter", used)
	}
}

func TestReactionPromptCarriesPersonality(t *testing.T) {
	p := ReactionPrompt("Stub", "PERSONA-MARKER", "digest", "summary")
	if !strings.Contains(p, "PERSONA-MARKER") {
		t.Error("reaction prompt missing personality")
	}
}

func TestGetDecisionPromptUsesGivenPrompt(t *testing.T) {
	var seen []string
	ask := func(ctx context.Context, p string) (Response, error) {
		seen = append(seen, p)
		return Response{Text: `{"action":"check"}`}, nil
	}
	data := RequestData{LegalActions: "check"}
	act, _, fb, _ := GetDecisionPrompt(context.Background(), ask, "DELTA-PROMPT", data,
		[]poker.ActionType{poker.Check})
	if fb || act.Type != poker.Check {
		t.Fatalf("decision failed: %v fb=%v", act, fb)
	}
	if len(seen) != 1 || seen[0] != "DELTA-PROMPT" {
		t.Errorf("ask saw %q, want the given prompt verbatim", seen)
	}
}

func TestGetReactionPromptUsesGivenPrompt(t *testing.T) {
	ask := func(ctx context.Context, p string) (Response, error) {
		if p != "REACT-PROMPT" {
			t.Errorf("prompt = %q", p)
		}
		return Response{
			Text:  "one line\nsecond",
			Usage: &Usage{InputTokens: 10, OutputTokens: 3, CostUSD: 0.001},
		}, nil
	}
	line, u := GetReactionPrompt(context.Background(), ask, "REACT-PROMPT")
	if line != "one line" {
		t.Errorf("line = %q", line)
	}
	if u == nil || u.InputTokens != 10 || u.OutputTokens != 3 {
		t.Errorf("usage = %+v, want InputTokens=10 OutputTokens=3", u)
	}
}
