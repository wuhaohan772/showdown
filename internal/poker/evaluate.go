package poker

import (
	"fmt"

	chp "github.com/chehsunliu/poker"
)

func toLib(cs []Card) []chp.Card {
	out := make([]chp.Card, len(cs))
	for i, c := range cs {
		out[i] = chp.NewCard(c.String())
	}
	return out
}

// Evaluate returns the strength of the best 5-card hand (lower = stronger)
// and a human description like "Flush".
func Evaluate(cs []Card) (int32, string) {
	if len(cs) < 5 || len(cs) > 7 {
		panic(fmt.Sprintf("poker.Evaluate: need 5-7 cards, got %d", len(cs)))
	}
	rank := chp.Evaluate(toLib(cs))
	return rank, chp.RankString(rank)
}

// CompareHands compares best 5-card hands; returns -1 if A wins, 1 if B wins, 0 on split.
func CompareHands(holeA, holeB [2]Card, board []Card) int {
	ra, _ := Evaluate(append(holeA[:], board...))
	rb, _ := Evaluate(append(holeB[:], board...))
	switch {
	case ra < rb:
		return -1
	case ra > rb:
		return 1
	}
	return 0
}
