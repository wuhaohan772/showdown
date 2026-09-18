package agent

import (
	_ "embed"
	"fmt"
	"strconv"
	"strings"

	"github.com/wuhaohan772/showdown/internal/poker"
)

//go:embed prompt_template.md
var promptTemplate string

type RequestData struct {
	AgentName    string
	Personality  string
	MatchDigest  string
	HandState    string
	TalkLog      string
	LegalActions string
	MinRaise     int
	MaxAmount    int
}

func RenderPrompt(d RequestData) string {
	r := strings.NewReplacer(
		"{agent_name}", d.AgentName,
		"{personality}", d.Personality,
		"{match_digest}", d.MatchDigest,
		"{hand_state}", d.HandState,
		"{talk_log}", d.TalkLog,
		"{legal_actions}", d.LegalActions,
		"{min_raise}", strconv.Itoa(d.MinRaise),
		"{max_amount}", strconv.Itoa(d.MaxAmount),
		"{amount_rules}", amountRules(d),
	)
	out := r.Replace(promptTemplate)
	out = strings.ReplaceAll(out, "{{", "{")
	out = strings.ReplaceAll(out, "}}", "}")
	return out
}

// raiseLegal reports whether this turn allows putting more chips in than a
// call — the only case where "amount" and the raise bounds mean anything.
func raiseLegal(legalActions string) bool {
	return strings.Contains(legalActions, "raise") || strings.Contains(legalActions, "bet")
}

// amountRules renders the "amount" format bullets for this turn. Quoting
// raise bounds when no raise is legal describes a move the agent cannot
// make, so that turn gets the short rule instead.
func amountRules(d RequestData) string {
	if !raiseLegal(d.LegalActions) {
		return `- Omit "amount" entirely: no bet or raise is legal this turn.`
	}
	return fmt.Sprintf(`- "amount" is required only for bet/raise. It is the TOTAL number of chips you are betting/raising TO (not the increment). Minimum %d, maximum %d (all-in).
- Omit "amount" for fold/check/call.`, d.MinRaise, d.MaxAmount)
}

// BuildHandState describes the hand from agentSeat's perspective (agent = "you"), regardless of whose turn it is.
func BuildHandState(h *poker.Hand, agentSeat int, sb, bb int) string {
	var b strings.Builder
	you, opp := h.Seats[agentSeat], h.Seats[1-agentSeat]
	pos := "BUTTON/SB (act first preflop, last postflop)"
	if agentSeat == 1 {
		pos = "BIG BLIND (act last preflop, first postflop)"
	}
	fmt.Fprintf(&b, "Blinds %d/%d. You are %s.\n", sb, bb, pos)
	fmt.Fprintf(&b, "Your stack: %d. Opponent stack: %d. Pot: %d.\n", you.Stack, opp.Stack, h.Pot)
	fmt.Fprintf(&b, "Your hole cards: %s %s\n", h.Hole[agentSeat][0], h.Hole[agentSeat][1])
	if len(h.Board) == 0 {
		b.WriteString("Board: (preflop)\n")
	} else {
		strs := make([]string, len(h.Board))
		for i, c := range h.Board {
			strs[i] = c.String()
		}
		fmt.Fprintf(&b, "Board: %s\n", strings.Join(strs, " "))
	}
	b.WriteString("Action so far:")
	if len(h.Log) == 0 {
		b.WriteString(" (blinds posted, no actions yet)")
	}
	for _, li := range h.Log {
		who := "opponent"
		if li.Seat == agentSeat {
			who = "you"
		}
		switch li.Act.Type {
		case poker.Fold, poker.Check, poker.Call:
			fmt.Fprintf(&b, " [%s] %s %s.", li.Street, who, li.Act.Type)
		default:
			allin := ""
			if li.AllIn {
				allin = " (ALL-IN)"
			}
			fmt.Fprintf(&b, " [%s] %s %s to %d%s.", li.Street, who, li.Act.Type, li.Act.To, allin)
		}
	}
	b.WriteString("\n")
	call := h.CurrentBet - h.Seats[agentSeat].Committed
	if call > h.Seats[agentSeat].Stack {
		call = h.Seats[agentSeat].Stack
	}
	if call > 0 {
		fmt.Fprintf(&b, "To you: call %d more, raise, or fold.", call)
	} else {
		b.WriteString("To you: check or bet/raise.")
	}
	return b.String()
}

// RenderDelta is the per-decision message for an already-primed session:
// only what changed since the last turn. handResults are digest summary
// lines for hands finished since the previous agent turn. The static
// blocks (persona, match intro, format rules) live in the session's
// priming turn and must not be repeated here — the conversation carries
// them, which is what keeps the prompt cache warm.
func RenderDelta(d RequestData, handResults []string) string {
	var b strings.Builder
	for _, r := range handResults {
		b.WriteString(r + "\n")
	}
	b.WriteString("\n=== CURRENT HAND ===\n" + d.HandState + "\n")
	b.WriteString("\n=== TABLE TALK THIS HAND ===\n" + d.TalkLog + "\n")
	fmt.Fprintf(&b, "\n=== YOUR MOVE ===\nLegal actions: %s\n", d.LegalActions)
	b.WriteString("Reply with ONLY the JSON object — same format and rules as before.")
	if raiseLegal(d.LegalActions) {
		fmt.Fprintf(&b, " Minimum raise-to %d, maximum %d (all-in).", d.MinRaise, d.MaxAmount)
	} else {
		b.WriteString(` No bet or raise is legal this turn, so omit "amount".`)
	}
	b.WriteString("\n")
	return b.String()
}
