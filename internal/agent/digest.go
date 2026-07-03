package agent

import "strings"

// Digest is the agent's stateless "memory": hand summaries plus table talk.
type Digest struct {
	events   []string
	handTalk []string
}

func NewDigest() *Digest { return &Digest{} }

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

// EndHand archives this hand's talk into the running digest and records the summary.
func (d *Digest) EndHand(summary string) {
	for _, t := range d.handTalk {
		d.events = append(d.events, "  talk — "+t)
	}
	d.handTalk = nil
	d.events = append(d.events, summary)
}

func (d *Digest) String() string {
	if len(d.events) == 0 {
		return "First hand of the match. Stacks even at 1500."
	}
	return strings.Join(d.events, "\n")
}
