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
	// If posting blinds already leaves the button all-in, the BB can gain
	// nothing by acting (an all-in player can never respond), so there is no
	// live decision preflop: refund the BB's excess and run the board out.
	if h.Seats[0].AllIn {
		h.refundExcess()
		h.dealRemainingAndShowdown()
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

type Result struct {
	Winner   int // seat index; -1 when Split
	Split    bool
	Pot      int
	Desc     string
	Showdown bool
}

func (h *Hand) Result() *Result { return h.result }

func (h *Hand) Apply(a Action) error {
	legal := false
	for _, t := range h.LegalActions() {
		if t == a.Type {
			legal = true
		}
	}
	if !legal {
		return fmt.Errorf("%s is not legal now (legal: %v)", a.Type, h.LegalActions())
	}
	s := &h.Seats[h.Actor]
	switch a.Type {
	case Fold:
		s.Folded = true
		h.Log = append(h.Log, LogItem{Seat: h.Actor, Street: h.Street, Act: a, AllIn: s.AllIn})
		h.finishFold(1 - h.Actor)
		return nil
	case Check:
		h.acted++
	case Call:
		h.post(h.Actor, h.CallAmount())
		h.acted++
	case Bet, Raise:
		if a.To > h.MaxRaiseTo() {
			return fmt.Errorf("raise to %d exceeds all-in max %d", a.To, h.MaxRaiseTo())
		}
		if a.To < h.MinRaiseTo() && a.To != h.MaxRaiseTo() {
			return fmt.Errorf("raise to %d below minimum %d", a.To, h.MinRaiseTo())
		}
		if a.To <= h.CurrentBet {
			return fmt.Errorf("raise to %d must exceed current bet %d", a.To, h.CurrentBet)
		}
		h.lastRaiseSize = a.To - h.CurrentBet
		h.post(h.Actor, a.To-s.Committed)
		h.CurrentBet = a.To
		h.acted = 1
	}
	h.Log = append(h.Log, LogItem{Seat: h.Actor, Street: h.Street, Act: a, AllIn: s.AllIn})
	if h.roundShouldClose() {
		h.refundExcess()
		h.nextStreet()
	} else {
		h.Actor = 1 - h.Actor
	}
	return nil
}

// roundShouldClose reports whether no further response is possible on the
// current street. In heads-up play this is either the usual "both matched
// and both have acted" case, or — whenever either seat is all-in — the case
// where the live seat has matched or exceeded the all-in seat's commitment
// (an all-in seat can never act again, so nothing more can happen).
func (h *Hand) roundShouldClose() bool {
	s0, s1 := h.Seats[0], h.Seats[1]
	switch {
	case s0.AllIn && s1.AllIn:
		return true
	case s0.AllIn:
		return s1.Committed >= s0.Committed
	case s1.AllIn:
		return s0.Committed >= s1.Committed
	default:
		return s0.Committed == s1.Committed && h.acted >= 2
	}
}

// refundExcess returns any uncalled excess to the deeper-committed seat
// whenever the street closes with unequal commitments — e.g. a call that is
// all-in for less, or an all-in seat whose opponent already committed more
// (from a blind post or an earlier call). Chips are always conserved.
func (h *Hand) refundExcess() {
	s0, s1 := &h.Seats[0], &h.Seats[1]
	var deep, shallow *SeatState
	switch {
	case s0.Committed > s1.Committed:
		deep, shallow = s0, s1
	case s1.Committed > s0.Committed:
		deep, shallow = s1, s0
	default:
		return
	}
	excess := deep.Committed - shallow.Committed
	deep.Stack += excess
	deep.Committed -= excess
	deep.Total -= excess
	h.Pot -= excess
	deep.AllIn = deep.Stack == 0
	h.CurrentBet = shallow.Committed
}

func (h *Hand) finishFold(winner int) {
	h.Street = HandOver
	h.result = &Result{Winner: winner, Pot: h.Pot}
	h.Seats[winner].Stack += h.Pot
}

func (h *Hand) nextStreet() {
	for i := range h.Seats {
		h.Seats[i].Committed = 0
	}
	h.CurrentBet = 0
	h.lastRaiseSize = h.bb
	h.acted = 0
	if h.Seats[0].AllIn || h.Seats[1].AllIn {
		h.dealRemainingAndShowdown()
		return
	}
	if h.Street < River {
		h.Street++
		h.dealStreet()
		h.Actor = 1 // BB acts first postflop
		return
	}
	h.showdown()
}

// dealStreet deals the correct number of board cards for the current street
// (3 on the flop, 1 otherwise).
func (h *Hand) dealStreet() {
	n := 3
	if h.Street != Flop {
		n = 1
	}
	for i := 0; i < n; i++ {
		h.Board = append(h.Board, h.deck.Deal())
	}
}

// dealRemainingAndShowdown deals every street up to the river (used both for
// a normal all-in runout and for the blind-post-all-in case at NewHand time)
// and then resolves the showdown.
func (h *Hand) dealRemainingAndShowdown() {
	for h.Street < River {
		h.Street++
		h.dealStreet()
	}
	h.showdown()
}

func (h *Hand) showdown() {
	h.Street = HandOver
	cmp := CompareHands(h.Hole[0], h.Hole[1], h.Board)
	_, desc0 := Evaluate(append(h.Hole[0][:], h.Board...))
	_, desc1 := Evaluate(append(h.Hole[1][:], h.Board...))
	switch cmp {
	case -1:
		h.result = &Result{Winner: 0, Pot: h.Pot, Desc: desc0, Showdown: true}
		h.Seats[0].Stack += h.Pot
	case 1:
		h.result = &Result{Winner: 1, Pot: h.Pot, Desc: desc1, Showdown: true}
		h.Seats[1].Stack += h.Pot
	default:
		h.result = &Result{Winner: -1, Split: true, Pot: h.Pot, Desc: desc0, Showdown: true}
		half := h.Pot / 2
		h.Seats[0].Stack += h.Pot - half // odd chip to the button
		h.Seats[1].Stack += half
	}
}
