package poker

import chp "github.com/chehsunliu/poker"

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
	rank := chp.Evaluate(toLib(cs))
	return rank, chp.RankString(rank)
}

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
