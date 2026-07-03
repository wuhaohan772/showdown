package agent

import (
	_ "embed"
	"fmt"
	"strconv"
	"strings"

	"github.com/haohanwu/showdown/internal/poker"
)

//go:embed prompt_template.md
var promptTemplate string

type RequestData struct {
	AgentName    string
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
		"{match_digest}", d.MatchDigest,
		"{hand_state}", d.HandState,
		"{talk_log}", d.TalkLog,
		"{legal_actions}", d.LegalActions,
		"{min_raise}", strconv.Itoa(d.MinRaise),
		"{max_amount}", strconv.Itoa(d.MaxAmount),
	)
	out := r.Replace(promptTemplate)
	out = strings.ReplaceAll(out, "{{", "{")
	out = strings.ReplaceAll(out, "}}", "}")
	return out
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
