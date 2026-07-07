package poker

import "math"

// blindFactors are the escalation ratios of the standard 10/20 schedule
// (10, 15, 25, 50, 100, 200 → 1x, 1.5x, 2.5x, 5x, 10x, 20x of the starting
// small blind). blindLevelsFor scales these to any starting small blind so
// the escalation feel stays the same regardless of buy-in size.
var blindFactors = []float64{1, 1.5, 2.5, 5, 10, 20}

func blindLevelsFor(startSB int) [][2]int {
	levels := make([][2]int, len(blindFactors))
	for i, f := range blindFactors {
		sb := int(math.Round(float64(startSB) * f))
		if sb < 1 {
			sb = 1
		}
		levels[i] = [2]int{sb, sb * 2}
	}
	return levels
}

// Match tracks the sit-and-go by player id: 0 = human, 1 = agent.
type Match struct {
	Stacks  [2]int
	HandNum int

	levels    [][2]int
	handLimit int // 0 = unlimited (bust-only); otherwise a hand cap
}

func NewMatch(startStack, startSB, handLimit int) *Match {
	return &Match{Stacks: [2]int{startStack, startStack}, HandNum: 1, levels: blindLevelsFor(startSB), handLimit: handLimit}
}

func (m *Match) Blinds() (int, int) {
	lvl := (m.HandNum - 1) / 10
	if lvl >= len(m.levels) {
		lvl = len(m.levels) - 1
	}
	return m.levels[lvl][0], m.levels[lvl][1]
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

// EndedByBust reports whether either player is out of chips. Distinguishes a
// bust ending from a hand-limit ending so callers (e.g. the agent's
// match-over reaction) can describe the true reason the match ended.
func (m *Match) EndedByBust() bool { return m.Stacks[0] == 0 || m.Stacks[1] == 0 }

// Over reports whether the match has ended: either someone busted, or the
// hand limit (if set) has been reached with stacks no longer tied. Tied
// stacks at the limit play on ("sudden death") until a hand breaks the tie.
func (m *Match) Over() bool {
	if m.EndedByBust() {
		return true
	}
	return m.handLimit > 0 && m.HandNum-1 >= m.handLimit && m.Stacks[0] != m.Stacks[1]
}

func (m *Match) Winner() int {
	switch {
	case !m.Over():
		return -1
	case m.Stacks[0] == 0:
		return 1
	case m.Stacks[1] == 0:
		return 0
	case m.Stacks[0] > m.Stacks[1]:
		return 0
	default:
		return 1
	}
}
