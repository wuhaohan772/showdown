package agent

import (
	"fmt"
	"strings"
)

// Digest is the agent's stateless "memory": hand summaries plus table talk.
type Digest struct {
	events     []string
	handTalk   []string
	startStack int

	handsPlayed    int
	opponentShoves int
	foldedToShove  int
}

func NewDigest(startStack int) *Digest { return &Digest{startStack: startStack} }

func (d *Digest) AddEvent(line string) { d.events = append(d.events, line) }

func (d *Digest) AddTalk(speaker, line string) {
	d.handTalk = append(d.handTalk, speaker+": "+line)
}

func (d *Digest) HandTalk() string {
	if len(d.handTalk) == 0 {
		return "(nothing said yet)"
	}
	return strings.Join(d.handTalk, "\n")
}

// RecordHand accumulates one finished hand's shove read (see DetectShove)
// into the running match-wide tendency counters.
func (d *Digest) RecordHand(sawShove, foldedToShove bool) {
	d.handsPlayed++
	if sawShove {
		d.opponentShoves++
	}
	if foldedToShove {
		d.foldedToShove++
	}
}

// EndHand archives this hand's talk into the running digest and records the summary.
func (d *Digest) EndHand(summary string) {
	for _, t := range d.handTalk {
		d.events = append(d.events, "  talk — "+t)
	}
	d.handTalk = nil
	d.events = append(d.events, summary)
}

func (d *Digest) String() string {
	base := strings.Join(d.events, "\n")
	if len(d.events) == 0 {
		base = fmt.Sprintf("First hand of the match. Stacks even at %d.", d.startStack)
	}
	if d.handsPlayed >= 3 && d.opponentShoves > 0 {
		tendency := fmt.Sprintf("Opponent tendency: shoved all-in %d of %d hands; you folded to %d of those shoves.",
			d.opponentShoves, d.handsPlayed, d.foldedToShove)
		return tendency + "\n" + base
	}
	return base
}
