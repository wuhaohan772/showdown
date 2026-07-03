package poker

import (
	"math/rand"
	"testing"
)

func newTestHand(t *testing.T) *Hand {
	t.Helper()
	return NewHand([2]int{1500, 1500}, 10, 20, rand.New(rand.NewSource(7)))
}

func hasAction(as []ActionType, a ActionType) bool {
	for _, x := range as {
		if x == a {
			return true
		}
	}
	return false
}

func TestNewHandPostsBlinds(t *testing.T) {
	h := newTestHand(t)
	if h.Seats[0].Stack != 1490 || h.Seats[0].Committed != 10 {
		t.Errorf("button/SB: stack %d committed %d, want 1490/10", h.Seats[0].Stack, h.Seats[0].Committed)
	}
	if h.Seats[1].Stack != 1480 || h.Seats[1].Committed != 20 {
		t.Errorf("BB: stack %d committed %d, want 1480/20", h.Seats[1].Stack, h.Seats[1].Committed)
	}
	if h.Pot != 30 || h.CurrentBet != 20 || h.Actor != 0 || h.Street != Preflop {
		t.Errorf("pot %d bet %d actor %d street %v", h.Pot, h.CurrentBet, h.Actor, h.Street)
	}
	if h.Hole[0] == h.Hole[1] {
		t.Error("both seats dealt identical hole cards")
	}
}

func TestPreflopLegalActionsAndAmounts(t *testing.T) {
	h := newTestHand(t)
	la := h.LegalActions()
	for _, want := range []ActionType{Fold, Call, Raise} {
		if !hasAction(la, want) {
			t.Errorf("preflop button missing %s in %v", want, la)
		}
	}
	if hasAction(la, Check) || hasAction(la, Bet) {
		t.Errorf("preflop button should not have check/bet: %v", la)
	}
	if h.CallAmount() != 10 {
		t.Errorf("CallAmount = %d, want 10", h.CallAmount())
	}
	if h.MinRaiseTo() != 40 {
		t.Errorf("MinRaiseTo = %d, want 40 (BB 20 + raise size 20)", h.MinRaiseTo())
	}
	if h.MaxRaiseTo() != 1500 {
		t.Errorf("MaxRaiseTo = %d, want 1500 (all-in total)", h.MaxRaiseTo())
	}
}

func TestAllInActorHasNoActions(t *testing.T) {
	// BB has only 15 and is all-in from posting the blind.
	h := NewHand([2]int{1500, 15}, 10, 20, rand.New(rand.NewSource(5)))
	h.Actor = 1
	if la := h.LegalActions(); la != nil {
		t.Errorf("all-in actor legal actions = %v, want nil", la)
	}
	h.Actor = 0
}

func mustApply(t *testing.T, h *Hand, a Action) {
	t.Helper()
	if err := h.Apply(a); err != nil {
		t.Fatalf("Apply(%v): %v", a, err)
	}
}

func TestFoldEndsHand(t *testing.T) {
	h := newTestHand(t)
	mustApply(t, h, Action{Type: Fold})
	r := h.Result()
	if r == nil || r.Winner != 1 || r.Showdown {
		t.Fatalf("result = %+v, want BB wins without showdown", r)
	}
	if r.Pot != 30 {
		t.Errorf("pot = %d, want 30", r.Pot)
	}
	if h.Street != HandOver {
		t.Errorf("street = %v, want HandOver", h.Street)
	}
}

func TestLimpCheckAdvancesToFlop(t *testing.T) {
	h := newTestHand(t)
	mustApply(t, h, Action{Type: Call}) // button limps
	if h.Actor != 1 || h.Street != Preflop {
		t.Fatalf("BB should have the option; actor %d street %v", h.Actor, h.Street)
	}
	mustApply(t, h, Action{Type: Check}) // BB checks option
	if h.Street != Flop || len(h.Board) != 3 {
		t.Fatalf("street %v board %d cards, want flop/3", h.Street, len(h.Board))
	}
	if h.Actor != 1 {
		t.Errorf("postflop first actor = %d, want 1 (BB out of position)", h.Actor)
	}
	if h.CurrentBet != 0 || h.Seats[0].Committed != 0 {
		t.Errorf("street commitments should reset: bet %d committed %d", h.CurrentBet, h.Seats[0].Committed)
	}
}

func TestRaiseCallFlow(t *testing.T) {
	h := newTestHand(t)
	mustApply(t, h, Action{Type: Raise, To: 60})
	if h.CurrentBet != 60 || h.Actor != 1 {
		t.Fatalf("after raise: bet %d actor %d", h.CurrentBet, h.Actor)
	}
	if h.MinRaiseTo() != 100 {
		t.Errorf("MinRaiseTo = %d, want 100 (60 + raise size 40)", h.MinRaiseTo())
	}
	mustApply(t, h, Action{Type: Call})
	if h.Street != Flop || h.Pot != 120 {
		t.Errorf("street %v pot %d, want Flop/120", h.Street, h.Pot)
	}
}

func TestIllegalActionsRejected(t *testing.T) {
	h := newTestHand(t)
	if err := h.Apply(Action{Type: Check}); err == nil {
		t.Error("check while facing a bet should fail")
	}
	if err := h.Apply(Action{Type: Raise, To: 30}); err == nil {
		t.Error("raise below minimum should fail")
	}
	if err := h.Apply(Action{Type: Raise, To: 9999}); err == nil {
		t.Error("raise above stack should fail")
	}
}

func TestBetThroughAllStreets(t *testing.T) {
	h := newTestHand(t)
	mustApply(t, h, Action{Type: Call})
	mustApply(t, h, Action{Type: Check}) // flop
	for _, street := range []Street{Turn, River, HandOver} {
		mustApply(t, h, Action{Type: Check}) // BB
		mustApply(t, h, Action{Type: Check}) // button
		if h.Street != street {
			t.Fatalf("street = %v, want %v", h.Street, street)
		}
	}
	if len(h.Board) != 5 || h.Result() == nil {
		t.Errorf("board %d, result %v — want 5 cards and a showdown result", len(h.Board), h.Result())
	}
}
