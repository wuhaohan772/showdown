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
