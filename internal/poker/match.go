package poker

var blindLevels = [][2]int{{10, 20}, {15, 30}, {25, 50}, {50, 100}, {100, 200}, {200, 400}}

// Match tracks the sit-and-go by player id: 0 = human, 1 = agent.
type Match struct {
	Stacks  [2]int
	HandNum int
}

func NewMatch() *Match {
	return &Match{Stacks: [2]int{1500, 1500}, HandNum: 1}
}

func (m *Match) Blinds() (int, int) {
	lvl := (m.HandNum - 1) / 10
	if lvl >= len(blindLevels) {
		lvl = len(blindLevels) - 1
	}
	return blindLevels[lvl][0], blindLevels[lvl][1]
}

func (m *Match) ButtonPlayer() int { return (m.HandNum - 1) % 2 }

// SeatOf maps a player id to a hand seat (seat 0 = button).
func (m *Match) SeatOf(player int) int {
	if player == m.ButtonPlayer() {
		return 0
	}
	return 1
}

func (m *Match) PlayerAt(seat int) int {
	if seat == 0 {
		return m.ButtonPlayer()
	}
	return 1 - m.ButtonPlayer()
}

// NextHand records final stacks (indexed by seat) and advances the hand counter.
func (m *Match) NextHand(finalSeatStacks [2]int) {
	m.Stacks[m.PlayerAt(0)] = finalSeatStacks[0]
	m.Stacks[m.PlayerAt(1)] = finalSeatStacks[1]
	m.HandNum++
}

func (m *Match) Over() bool { return m.Stacks[0] == 0 || m.Stacks[1] == 0 }

func (m *Match) Winner() int {
	switch {
	case !m.Over():
		return -1
	case m.Stacks[0] == 0:
		return 1
	}
	return 0
}
