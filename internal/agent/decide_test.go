package agent

import (
	"context"
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/haohanwu/showdown/internal/poker"
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
	garbage := func(ctx context.Context, prompt string) (string, error) {
		calls++
		return "I love poker!!!", nil
	}
	a, _, fb := GetDecision(context.Background(), garbage, RequestData{}, legalFCR())
	if !fb || a.Type != poker.Fold {
		t.Errorf("garbage twice → fallback fold; got %+v fb=%v", a, fb)
	}
	if calls != 2 {
		t.Errorf("want exactly 2 calls (one retry), got %d", calls)
	}
}

func TestGetDecisionRetrySucceeds(t *testing.T) {
	calls := 0
	flaky := func(ctx context.Context, prompt string) (string, error) {
		calls++
		if calls == 1 {
			return "hmm let me think", nil
		}
		return `{"action": "call", "say": "fine"}`, nil
	}
	a, say, fb := GetDecision(context.Background(), flaky, RequestData{}, legalFCR())
	if fb || a.Type != poker.Call || say != "fine" {
		t.Errorf("got %+v say=%q fb=%v", a, say, fb)
	}
}

func TestGetDecisionErrorNoRetry(t *testing.T) {
	calls := 0
	dead := func(ctx context.Context, prompt string) (string, error) {
		calls++
		return "", errors.New("timeout")
	}
	a, _, fb := GetDecision(context.Background(), dead, RequestData{}, legalFCR())
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
	ask := func(ctx context.Context, prompt string) (string, error) { return long, nil }
	got := GetReaction(context.Background(), ask, "X", "d", "s")
	if !utf8.ValidString(got) {
		t.Errorf("truncated reaction is invalid UTF-8: %q", got)
	}
	if utf8.RuneCountInString(got) > 120 {
		t.Errorf("rune count %d > 120", utf8.RuneCountInString(got))
	}
}
