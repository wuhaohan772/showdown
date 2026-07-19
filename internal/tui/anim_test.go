package tui

import "testing"

func TestStepToward(t *testing.T) {
	cases := []struct{ cur, target, want int }{
		{0, 100, 25},   // quarter of the distance
		{99, 100, 100}, // small gaps still move (min step 1)
		{100, 0, 75},   // works downward
		{100, 99, 99},  // min step -1
		{5, 5, 5},      // at target: no move
	}
	for _, c := range cases {
		if got := stepToward(c.cur, c.target); got != c.want {
			t.Errorf("stepToward(%d,%d) = %d, want %d", c.cur, c.target, got, c.want)
		}
	}
}

func TestCardSlideSequence(t *testing.T) {
	s := newCardSlide(0, 2)
	if !s.active() {
		t.Fatal("fresh slide must be active")
	}
	// Card 1: offsets 12 (initial), then 8, 4, 0 after ticks; next tick lands it.
	wantOffsets := []int{8, 4, 0}
	for i, w := range wantOffsets {
		s.tick()
		if s.offset != w || s.landed != 0 {
			t.Fatalf("tick %d: offset=%d landed=%d, want offset=%d landed=0", i+1, s.offset, s.landed, w)
		}
	}
	s.tick() // lands card 1, card 2 starts at slideStart
	if s.landed != 1 || s.offset != slideStart {
		t.Fatalf("after landing: landed=%d offset=%d, want 1/%d", s.landed, s.offset, slideStart)
	}
	for s.active() {
		s.tick()
	}
	if s.landed != 2 {
		t.Errorf("finished slide landed=%d, want 2", s.landed)
	}
	s.tick() // ticking an inactive slide is a no-op
	if s.landed != 2 {
		t.Errorf("tick after done changed state: %+v", s)
	}
}

func TestCardSlideStartsPartial(t *testing.T) {
	s := newCardSlide(3, 5) // turn/river: board already has 3 landed
	if s.landed != 3 || !s.active() {
		t.Fatalf("partial slide: landed=%d active=%v, want 3/true", s.landed, s.active())
	}
}

func TestZeroValueCardSlideInactive(t *testing.T) {
	var s cardSlide
	if s.active() {
		t.Error("zero-value cardSlide must be inactive")
	}
}
