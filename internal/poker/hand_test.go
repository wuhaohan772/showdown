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

// --- All-in edge cases (Task 5 gaps A/B/C) ---

func TestButtonAllInFromBlind_ImmediateRunout(t *testing.T) {
	// Button/SB has only 5 chips, less than the 10 SB: posting the blind
	// leaves the button all-in before anyone has a decision. The BB gains
	// nothing by acting (button can never respond), so the hand must
	// resolve immediately at NewHand time.
	h := NewHand([2]int{5, 1500}, 10, 20, rand.New(rand.NewSource(11)))
	if h.Street != HandOver {
		t.Fatalf("street = %v, want HandOver (immediate runout)", h.Street)
	}
	r := h.Result()
	if r == nil || !r.Showdown {
		t.Fatalf("result = %+v, want a showdown result", r)
	}
	if len(h.Board) != 5 {
		t.Errorf("board = %d cards, want 5", len(h.Board))
	}
	if got, want := h.Seats[0].Stack+h.Seats[1].Stack, 5+1500; got != want {
		t.Errorf("chips not conserved: total stacks = %d, want %d", got, want)
	}
}

func TestBBAllInFromBlindThenButtonCalls_RunoutAndClose(t *testing.T) {
	// BB has only 15, less than the 20 BB: posting leaves BB all-in, but
	// the button (not all-in) still gets a normal decision (fold/call).
	h := NewHand([2]int{1500, 15}, 10, 20, rand.New(rand.NewSource(11)))
	if h.Street != Preflop {
		t.Fatalf("street = %v, want Preflop (button still has a live decision)", h.Street)
	}
	mustApply(t, h, Action{Type: Call})
	if h.Street != HandOver {
		t.Fatalf("street = %v, want HandOver after button calls the all-in BB", h.Street)
	}
	r := h.Result()
	if r == nil || !r.Showdown {
		t.Fatalf("result = %+v, want a showdown result", r)
	}
	if len(h.Board) != 5 {
		t.Errorf("board = %d cards, want 5", len(h.Board))
	}
	if r.Pot != 30 {
		t.Errorf("pot = %d, want 30 (both matched at 15, no refund due)", r.Pot)
	}
	if got, want := h.Seats[0].Stack+h.Seats[1].Stack, 1500+15; got != want {
		t.Errorf("chips not conserved: total stacks = %d, want %d", got, want)
	}
}

func TestButtonCommittedExceedsShortBBAllIn_ForcedCheckClosesWithRefund(t *testing.T) {
	// BB has only 5, less than even the 10 SB the button already posted.
	// The button's only legal action is Check (nothing to call, and no
	// raise is offered against an all-in opponent). That check must close
	// the street immediately and refund the button's excess over BB's 5.
	h := NewHand([2]int{1500, 5}, 10, 20, rand.New(rand.NewSource(11)))
	if h.Street != Preflop {
		t.Fatalf("street = %v, want Preflop", h.Street)
	}
	la := h.LegalActions()
	if len(la) != 1 || la[0] != Check {
		t.Fatalf("legal actions = %v, want only [Check]", la)
	}
	mustApply(t, h, Action{Type: Check})
	if h.Street != HandOver {
		t.Fatalf("street = %v, want HandOver after the forced check closes the round", h.Street)
	}
	r := h.Result()
	if r == nil || !r.Showdown {
		t.Fatalf("result = %+v, want a showdown result", r)
	}
	if len(h.Board) != 5 {
		t.Errorf("board = %d cards, want 5", len(h.Board))
	}
	if r.Pot != 10 {
		t.Errorf("pot = %d, want 10 (5+5 after refunding the button's excess)", r.Pot)
	}
	if got, want := h.Seats[0].Stack+h.Seats[1].Stack, 1500+5; got != want {
		t.Errorf("chips not conserved: total stacks = %d, want %d", got, want)
	}
}

// --- Task 6: additional all-in runout / refund / min-raise-exception coverage ---

func TestAllInCallRunsOutBoard(t *testing.T) {
	// Both seats start with equal 1500 stacks: the button open-shoves and the
	// BB calls off for exactly the same amount, so both seats end up all-in
	// with no refund due. This is the symmetric double all-in case, distinct
	// from the short-stack/refund scenarios covered elsewhere.
	h := newTestHand(t)
	mustApply(t, h, Action{Type: Raise, To: 1500}) // button open-shoves
	if !h.Seats[0].AllIn {
		t.Fatal("button should be all-in")
	}
	mustApply(t, h, Action{Type: Call}) // BB calls all-in
	if h.Street != HandOver || len(h.Board) != 5 {
		t.Fatalf("street %v board %d — want full runout to showdown", h.Street, len(h.Board))
	}
	r := h.Result()
	if r == nil || !r.Showdown {
		t.Fatal("want a showdown result")
	}
	if got := h.Seats[0].Stack + h.Seats[1].Stack; got != 3000 {
		t.Errorf("chips not conserved: %d", got)
	}
}

func TestBelowMinRaiseAllInAllowed(t *testing.T) {
	// Stack of 35 can shove even though min raise-to is 40.
	h := NewHand([2]int{35, 1500}, 10, 20, rand.New(rand.NewSource(3)))
	if err := h.Apply(Action{Type: Raise, To: 35}); err != nil {
		t.Fatalf("all-in below min raise should be legal: %v", err)
	}
}

func TestChipConservationRandomPlay(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	for i := 0; i < 500; i++ {
		// Widen the range below SB(10)/BB(20) so blind-post all-ins are hammered.
		stacks := [2]int{5 + rng.Intn(2000), 5 + rng.Intn(2000)}
		total := stacks[0] + stacks[1]
		h := NewHand(stacks, 10, 20, rng)
		for h.Street != HandOver {
			la := h.LegalActions()
			a := Action{Type: la[rng.Intn(len(la))]}
			if a.Type == Bet || a.Type == Raise {
				lo, hi := h.MinRaiseTo(), h.MaxRaiseTo()
				a.To = lo + rng.Intn(hi-lo+1)
			}
			if err := h.Apply(a); err != nil {
				t.Fatalf("iter %d: Apply(%+v): %v", i, a, err)
			}
		}
		if got := h.Seats[0].Stack + h.Seats[1].Stack; got != total {
			t.Fatalf("iter %d: chips %d, want %d", i, got, total)
		}
	}
}

func TestBetCalledByShorterAllInStack_Refund(t *testing.T) {
	// BB only has 100 total. Button shoves a big raise; BB can only call
	// all-in for less. The street must close and refund the button's
	// uncalled excess.
	h := NewHand([2]int{1500, 100}, 10, 20, rand.New(rand.NewSource(11)))
	mustApply(t, h, Action{Type: Raise, To: 500})
	if h.Street != Preflop {
		t.Fatalf("street = %v, want Preflop (BB still to act)", h.Street)
	}
	mustApply(t, h, Action{Type: Call})
	if h.Street != HandOver {
		t.Fatalf("street = %v, want HandOver after the short all-in call", h.Street)
	}
	r := h.Result()
	if r == nil || !r.Showdown {
		t.Fatalf("result = %+v, want a showdown result", r)
	}
	if r.Pot != 200 {
		t.Errorf("pot = %d, want 200 (100+100 after refunding the button's excess)", r.Pot)
	}
	if got, want := h.Seats[0].Stack+h.Seats[1].Stack, 1500+100; got != want {
		t.Errorf("chips not conserved: total stacks = %d, want %d", got, want)
	}
}

func TestShortAllInCallRefundsExcess(t *testing.T) {
	// BB is short: shove 1500 vs 600 stack — 900 must come back to the bettor.
	h := NewHand([2]int{1500, 600}, 10, 20, rand.New(rand.NewSource(9)))
	mustApply(t, h, Action{Type: Raise, To: 1500})
	mustApply(t, h, Action{Type: Call})
	if h.Pot != 1200 {
		t.Errorf("pot = %d, want 1200 (600 each)", h.Pot)
	}
	if got := h.Seats[0].Stack + h.Seats[1].Stack; got != 2100 {
		t.Errorf("chips not conserved: %d", got)
	}
	r := h.Result()
	if r == nil || r.Pot != 1200 {
		t.Fatalf("result %+v, want pot 1200", r)
	}
}
