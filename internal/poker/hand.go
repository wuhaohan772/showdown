package poker

import (
	"fmt"
	"math/rand"
)

type Street int

const (
	Preflop Street = iota
	Flop
	Turn
	River
	HandOver
)

func (s Street) String() string {
	return [...]string{"preflop", "flop", "turn", "river", "over"}[s]
}

type ActionType string

const (
	Fold  ActionType = "fold"
	Check ActionType = "check"
	Call  ActionType = "call"
	Bet   ActionType = "bet"
	Raise ActionType = "raise"
)

// Action.To is the TOTAL committed to this street for bet/raise; ignored otherwise.
type Action struct {
	Type ActionType
	To   int
}

type SeatState struct {
	Stack     int
	Committed int // this street
	Total     int // whole hand
	Folded    bool
	AllIn     bool
}

type LogItem struct {
	Seat   int
	Street Street
	Act    Action
	AllIn  bool
}

// Hand is one heads-up hand. Seat 0 = button/SB, seat 1 = BB.
type Hand struct {
	Seats      [2]SeatState
	Hole       [2][2]Card
	Board      []Card
	Street     Street
	Actor      int
	Pot        int
	CurrentBet int
	Log        []LogItem

	deck          *Deck
	lastRaiseSize int
	acted         int // players who acted since last aggression, this street
	bb            int
	result        *Result
}

func NewHand(stacks [2]int, sb, bb int, rng *rand.Rand) *Hand {
	h := &Hand{deck: NewDeck(rng), bb: bb, Street: Preflop, Actor: 0}
	h.Seats[0] = SeatState{Stack: stacks[0]}
	h.Seats[1] = SeatState{Stack: stacks[1]}
	h.post(0, min(sb, stacks[0]))
	h.post(1, min(bb, stacks[1]))
	h.CurrentBet = h.Seats[1].Committed
	h.lastRaiseSize = bb
	for i := 0; i < 2; i++ {
		h.Hole[i] = [2]Card{h.deck.Deal(), h.deck.Deal()}
	}
	return h
}

func (h *Hand) post(seat, n int) {
	s := &h.Seats[seat]
	s.Stack -= n
	s.Committed += n
	s.Total += n
	h.Pot += n
	if s.Stack == 0 {
		s.AllIn = true
	}
}

// CallAmount is the extra the actor must put in to call, capped at stack.
func (h *Hand) CallAmount() int {
	s := h.Seats[h.Actor]
	return min(h.CurrentBet-s.Committed, s.Stack)
}

func (h *Hand) MinRaiseTo() int {
	return min(h.CurrentBet+h.lastRaiseSize, h.MaxRaiseTo())
}

// MaxRaiseTo is the actor's all-in total for this street.
func (h *Hand) MaxRaiseTo() int {
	s := h.Seats[h.Actor]
	return s.Committed + s.Stack
}

func (h *Hand) LegalActions() []ActionType {
	if h.Street == HandOver {
		return nil
	}
	if h.Seats[h.Actor].AllIn || h.Seats[h.Actor].Folded {
		return nil
	}
	opp := h.Seats[1-h.Actor]
	var out []ActionType
	if h.CallAmount() > 0 {
		out = append(out, Fold, Call)
		if !opp.AllIn && h.MaxRaiseTo() > h.CurrentBet {
			out = append(out, Raise)
		}
	} else {
		out = append(out, Check)
		if !opp.AllIn {
			if h.CurrentBet == 0 {
				out = append(out, Bet)
			} else {
				out = append(out, Raise) // preflop BB option over a limp
			}
		}
	}
	return out
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

type Result struct{} // TODO: define in later task

var _ = fmt.Sprintf // placeholder use; removed when Task 5 adds Apply errors
