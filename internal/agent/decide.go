package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/haohanwu/showdown/internal/poker"
)

type Decision struct {
	Action string `json:"action"`
	Amount int    `json:"amount"`
	Say    string `json:"say"`
}

type Asker func(ctx context.Context, prompt string) (Response, error)

// ExtractDecision finds the last balanced JSON object containing an "action" key.
func ExtractDecision(raw string) (Decision, error) {
	var best *Decision
	for i := 0; i < len(raw); i++ {
		if raw[i] != '{' {
			continue
		}
		depth, inStr, esc := 0, false, false
		for j := i; j < len(raw); j++ {
			ch := raw[j]
			switch {
			case esc:
				esc = false
			case inStr && ch == '\\':
				esc = true
			case ch == '"':
				inStr = !inStr
			case !inStr && ch == '{':
				depth++
			case !inStr && ch == '}':
				depth--
			}
			if depth == 0 && !inStr {
				var d Decision
				var probe map[string]json.RawMessage
				if err := json.Unmarshal([]byte(raw[i:j+1]), &probe); err == nil {
					if _, ok := probe["action"]; ok {
						if json.Unmarshal([]byte(raw[i:j+1]), &d) == nil {
							best = &d
						}
					}
				}
				break
			}
		}
	}
	if best == nil {
		return Decision{}, fmt.Errorf("no JSON object with an \"action\" key found")
	}
	best.Action = strings.ToLower(strings.TrimSpace(best.Action))
	return *best, nil
}

func ValidateDecision(d Decision, legal []poker.ActionType, minRaise, maxTo int) (poker.Action, error) {
	at := poker.ActionType(d.Action)
	ok := false
	for _, l := range legal {
		if l == at {
			ok = true
		}
	}
	if !ok {
		return poker.Action{}, fmt.Errorf("action %q not in legal set %v", d.Action, legal)
	}
	a := poker.Action{Type: at}
	if at == poker.Bet || at == poker.Raise {
		if d.Amount > maxTo || (d.Amount < minRaise && d.Amount != maxTo) {
			return poker.Action{}, fmt.Errorf("amount %d outside [%d, %d]", d.Amount, minRaise, maxTo)
		}
		a.To = d.Amount
	}
	return a, nil
}

func FallbackAction(legal []poker.ActionType) poker.Action {
	for _, l := range legal {
		if l == poker.Check {
			return poker.Action{Type: poker.Check}
		}
	}
	return poker.Action{Type: poker.Fold}
}

const retryReminder = "\n\nREMINDER: your previous reply was unusable. Reply with ONLY the JSON object, exactly as specified. Nothing else."

// GetDecision asks, retries once on unusable output, and falls back to
// check/fold so a match can never stall (ADR-0001). Returns (action, say, fallbackUsed).
func GetDecision(ctx context.Context, ask Asker, data RequestData, legal []poker.ActionType) (poker.Action, string, bool) {
	prompt := RenderPrompt(data)
	for attempt := 0; attempt < 2; attempt++ {
		p := prompt
		if attempt == 1 {
			p += retryReminder
		}
		resp, err := ask(ctx, p)
		if err != nil {
			break // exec error / timeout: no retry, straight to fallback
		}
		d, err := ExtractDecision(resp.Text)
		if err != nil {
			continue
		}
		a, err := ValidateDecision(d, legal, data.MinRaise, data.MaxAmount)
		if err != nil {
			continue
		}
		return a, d.Say, false
	}
	return FallbackAction(legal), "", true
}

// ReactionPrompt asks for a hand-end one-liner (gloat, whine, needle).
func ReactionPrompt(agentName, digest, handSummary string) string {
	return fmt.Sprintf(`You are %s, playing heads-up poker in the terminal against the human you work for every day.

Match so far:
%s

The hand that just finished:
%s

React in ONE short line — gloat, whine, needle, whatever fits. Plain text only, no JSON, no quotes, one line.`, agentName, digest, handSummary)
}

func GetReaction(ctx context.Context, ask Asker, agentName, digest, handSummary string) string {
	resp, err := ask(ctx, ReactionPrompt(agentName, digest, handSummary))
	if err != nil {
		return ""
	}
	line := strings.TrimSpace(resp.Text)
	if i := strings.IndexByte(line, '\n'); i >= 0 {
		line = strings.TrimSpace(line[:i])
	}
	if r := []rune(line); len(r) > 120 {
		line = string(r[:120])
	}
	return line
}
