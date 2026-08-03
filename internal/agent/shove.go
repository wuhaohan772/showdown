package agent

import "github.com/wuhaohan772/showdown/internal/poker"

// DetectShove classifies a finished hand's log from agentSeat's perspective:
// sawShove is true if the opponent went all-in at any point in the hand;
// foldedToShove is true if that hand also ended with the agent folding.
func DetectShove(log []poker.LogItem, agentSeat int, r *poker.Result) (sawShove, foldedToShove bool) {
	for _, li := range log {
		if li.Seat != agentSeat && li.AllIn {
			sawShove = true
			break
		}
	}
	if sawShove && !r.Showdown && !r.Split && r.Winner != agentSeat {
		foldedToShove = true
	}
	return sawShove, foldedToShove
}
