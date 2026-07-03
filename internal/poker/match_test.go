package poker

import "testing"

func TestBlindSchedule(t *testing.T) {
	m := NewMatch()
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

func TestButtonAlternates(t *testing.T) {
	m := NewMatch()
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
	m := NewMatch() // hand 1: human is button = seat 0
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
}
