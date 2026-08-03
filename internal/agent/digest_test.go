package agent

import (
	"strings"
	"testing"
)

func TestDigestTendencyLineOmittedBelowThreshold(t *testing.T) {
	d := NewDigest(3000)
	d.RecordHand(true, true) // hand 1: shove, agent folded
	d.RecordHand(true, true) // hand 2: shove, agent folded
	if strings.Contains(d.String(), "Opponent tendency") {
		t.Errorf("digest with only 2 hands recorded should omit tendency line, got %q", d.String())
	}
}

func TestDigestTendencyLineOmittedWhenNoShoves(t *testing.T) {
	d := NewDigest(3000)
	for i := 0; i < 5; i++ {
		d.RecordHand(false, false)
	}
	if strings.Contains(d.String(), "Opponent tendency") {
		t.Errorf("digest with zero shoves should omit tendency line, got %q", d.String())
	}
}

func TestDigestTendencyLineAtThreshold(t *testing.T) {
	d := NewDigest(3000)
	d.RecordHand(true, true)  // shove, folded
	d.RecordHand(true, false) // shove, called
	d.RecordHand(false, false)
	want := "Opponent tendency: shoved all-in 2 of 3 hands; you folded to 1 of those shoves."
	if !strings.Contains(d.String(), want) {
		t.Errorf("digest = %q, want it to contain %q", d.String(), want)
	}
}

func TestDigestTendencyLineUpdatesAcrossMoreHands(t *testing.T) {
	d := NewDigest(3000)
	for i := 0; i < 6; i++ {
		d.RecordHand(true, i < 4) // 6 shoves, 4 folds
	}
	want := "Opponent tendency: shoved all-in 6 of 6 hands; you folded to 4 of those shoves."
	if !strings.Contains(d.String(), want) {
		t.Errorf("digest = %q, want it to contain %q", d.String(), want)
	}
}
