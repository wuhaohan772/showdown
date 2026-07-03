package poker

import "testing"

func cards(ss ...string) []Card {
	out := make([]Card, len(ss))
	for i, s := range ss {
		c, err := ParseCard(s)
		if err != nil {
			panic(err)
		}
		out[i] = c
	}
	return out
}

func hole(a, b string) [2]Card {
	c := cards(a, b)
	return [2]Card{c[0], c[1]}
}

func TestCompareHands(t *testing.T) {
	board := cards("Kh", "8h", "3d", "6c", "2h")
	// nut flush beats top pair
	if got := CompareHands(hole("Ah", "Jh"), hole("Ks", "Qd"), board); got != -1 {
		t.Errorf("flush vs pair = %d, want -1", got)
	}
	if got := CompareHands(hole("Ks", "Qd"), hole("Ah", "Jh"), board); got != 1 {
		t.Errorf("pair vs flush = %d, want 1", got)
	}
	// identical strength -> split (both play the board pair + kickers)
	split := cards("Ah", "Kh", "Kd", "7c", "2s")
	if got := CompareHands(hole("3c", "3d"), hole("3h", "3s"), split); got != 0 {
		t.Errorf("split = %d, want 0", got)
	}
}

func TestEvaluateDescription(t *testing.T) {
	_, desc := Evaluate(cards("Ah", "Jh", "Kh", "8h", "3d", "6c", "2h"))
	if desc == "" {
		t.Error("want non-empty description")
	}
}
