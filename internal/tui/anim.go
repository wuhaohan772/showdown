// Package-internal animation primitives. Pure state machines, no bubbletea:
// the single animTick loop in app.go advances them once per frame.
package tui

import "time"

const (
	animFrame  = 50 * time.Millisecond
	slideStart = 12 // columns right of the slot a dealt card starts at
	slideStep  = 4  // columns moved left per frame
)

// stepToward moves cur a quarter of the way to target (minimum 1), giving
// count-up/count-down tweens an ease-out feel. cur == target returns cur.
func stepToward(cur, target int) int {
	d := target - cur
	if d == 0 {
		return cur
	}
	step := d / 4
	if step == 0 {
		if d > 0 {
			step = 1
		} else {
			step = -1
		}
	}
	return cur + step
}

// cardSlide animates cards landing one at a time into a row. landed counts
// fully-arrived cards; offset is the extra indent of the currently sliding
// card. The zero value is inactive.
type cardSlide struct {
	total, landed, offset int
}

func newCardSlide(from, total int) cardSlide {
	return cardSlide{total: total, landed: from, offset: slideStart}
}

func (s cardSlide) active() bool { return s.landed < s.total }

func (s *cardSlide) tick() {
	if !s.active() {
		return
	}
	s.offset -= slideStep
	if s.offset < 0 {
		s.landed++
		s.offset = slideStart
	}
}
