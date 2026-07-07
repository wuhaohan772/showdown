package poker

import "testing"

func TestBlindSchedule(t *testing.T) {
	m := NewMatch(1500, 10, 0)
	cases := []struct{ hand, sb, bb int }{
		{1, 10, 20}, {10, 10, 20}, {11, 15, 30}, {21, 25, 50},
		{31, 50, 100}, {41, 100, 200}, {51, 200, 400}, {99, 200, 400},
	}
	for _, c := range cases {
		m.HandNum = c.hand
		sb, bb := m.Blinds()
		if sb != c.sb || bb != c.bb {
			t.Errorf("hand %d: blinds %d/%d, want %d/%d", c.hand, sb, bb, c.sb, c.bb)
		}
	}
}

// TestBlindScheduleScalesWithStartSB checks a non-default starting small
// blind (25) scales the whole 6-level schedule by the same ratios as the
// 10/20 default (1x, 1.5x, 2.5x, 5x, 10x, 20x), rounding to the nearest chip.
func TestBlindScheduleScalesWithStartSB(t *testing.T) {
	m := NewMatch(3000, 25, 0)
	cases := []struct{ hand, sb, bb int }{
		{1, 25, 50}, {11, 38, 76}, {21, 63, 126},
		{31, 125, 250}, {41, 250, 500}, {51, 500, 1000}, {99, 500, 1000},
	}
	for _, c := range cases {
		m.HandNum = c.hand
		sb, bb := m.Blinds()
		if sb != c.sb || bb != c.bb {
			t.Errorf("hand %d: blinds %d/%d, want %d/%d", c.hand, sb, bb, c.sb, c.bb)
		}
	}
}

func TestNewMatchCustomStartingStack(t *testing.T) {
	m := NewMatch(3000, 25, 0)
	if m.Stacks != [2]int{3000, 3000} {
		t.Errorf("Stacks = %v, want [3000 3000]", m.Stacks)
	}
}

func TestButtonAlternates(t *testing.T) {
	m := NewMatch(1500, 10, 0)
	if m.ButtonPlayer() != 0 {
		t.Errorf("hand 1 button = %d, want 0 (human)", m.ButtonPlayer())
	}
	m.HandNum = 2
	if m.ButtonPlayer() != 1 {
		t.Errorf("hand 2 button = %d, want 1", m.ButtonPlayer())
	}
	if m.SeatOf(m.ButtonPlayer()) != 0 {
		t.Error("button player must sit in seat 0")
	}
	if m.PlayerAt(m.SeatOf(0)) != 0 || m.PlayerAt(m.SeatOf(1)) != 1 {
		t.Error("SeatOf/PlayerAt must be inverses")
	}
}

func TestNextHandAndBust(t *testing.T) {
	m := NewMatch(1500, 10, 0) // hand 1: human is button = seat 0
	m.NextHand([2]int{3000, 0})
	if m.HandNum != 2 {
		t.Errorf("HandNum = %d, want 2", m.HandNum)
	}
	if m.Stacks[0] != 3000 || m.Stacks[1] != 0 {
		t.Errorf("stacks = %v, want human 3000 / agent 0", m.Stacks)
	}
	if !m.Over() || m.Winner() != 0 {
		t.Errorf("Over %v Winner %d, want true/0", m.Over(), m.Winner())
	}
	if !m.EndedByBust() {
		t.Error("EndedByBust() should be true: agent stack is 0")
	}
}

func TestHandLimitZeroMeansUnlimited(t *testing.T) {
	m := NewMatch(1500, 10, 0)
	m.HandNum = 500
	m.Stacks = [2]int{1000, 2000}
	if m.Over() {
		t.Error("handLimit=0 should mean unlimited hands; match should only end on bust")
	}
}

func TestHandLimitEndsMatchWhenStacksDiffer(t *testing.T) {
	m := NewMatch(1500, 10, 5)
	m.HandNum = 6 // 5 hands completed
	m.Stacks = [2]int{1800, 1200}
	if !m.Over() {
		t.Fatal("match should be over: hand limit reached with unequal stacks")
	}
	if m.Winner() != 0 {
		t.Errorf("Winner() = %d, want 0 (higher stack)", m.Winner())
	}
	if m.EndedByBust() {
		t.Error("EndedByBust() should be false: match ended by hand limit, not bust")
	}
}

func TestHandLimitSuddenDeathContinuesWhileTied(t *testing.T) {
	m := NewMatch(1500, 10, 5)
	m.HandNum = 6
	m.Stacks = [2]int{1500, 1500}
	if m.Over() {
		t.Fatal("tied stacks at the hand limit should continue (sudden death), not end")
	}
	m.HandNum = 7
	m.Stacks = [2]int{1600, 1400}
	if !m.Over() {
		t.Fatal("match should end once a sudden-death hand breaks the tie")
	}
	if m.Winner() != 0 {
		t.Errorf("Winner() = %d, want 0 (higher stack)", m.Winner())
	}
}

func TestHandLimitDoesNotOverrideBust(t *testing.T) {
	m := NewMatch(1500, 10, 100)
	m.HandNum = 3 // well before the limit
	m.Stacks = [2]int{3000, 0}
	if !m.Over() {
		t.Error("bust should end the match immediately regardless of hand limit")
	}
	if m.Winner() != 0 {
		t.Errorf("Winner() = %d, want 0", m.Winner())
	}
	if !m.EndedByBust() {
		t.Error("EndedByBust() should be true")
	}
}
