package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// A mascot sits on the opponent's side of the table for agents that have
// one, laid out like Claude Code's startup banner: three rows of art with
// one line of text beside each.
//
// Clawd (Claude Code) uses Claude Code's own glyph rows, clawd_body color
// and click animations (v2.1.282). The Codex prompt bot is ours; it is not
// OpenAI's logo.
type mascot struct {
	poses map[string][3]string // pose -> art rows
	style *lipgloss.Style      // points at a theme-owned style
}

var mascots = map[string]mascot{
	"claude": {style: &clawdStyle, poses: map[string][3]string{
		"default":    {" ▐▛███▛█", "▝▜██████▀", " ▝▝   ▝▝"},
		"look-left":  {" ▐▟███▟█", "▝▜██████▀", " ▝▝   ▝▝"},
		"look-right": {" ▐█▟███▟", "▝▜██████▀", " ▝▝   ▝▝"},
		"arms-up":    {"▗▟▛███▛█▄", " ▜██████▘", " ▝▝   ▝▝"},
	}},
	// same footprint as clawd: the box spans clawd's head, columns 1-7
	"codex": {style: &codexStyle, poses: map[string][3]string{
		"default":    {" ╭─────╮", " │ >_  │", " ╰─────╯"},
		"look-left":  {" ╭─────╮", " │>_   │", " ╰─────╯"},
		"look-right": {" ╭─────╮", " │   >_│", " ╰─────╯"},
		"arms-up":    {" ╭─────╮", " │ ^_^ │", " ╰─────╯"},
	}},
}

// mascotFor returns the opponent's mascot, if it has one.
func mascotFor(key string) (mascot, bool) {
	mc, ok := mascots[key]
	return mc, ok
}

// art returns the three glyph rows for a pose, unstyled. An unknown pose
// falls back to default rather than drawing nothing.
func (mc mascot) art(pose string) []string {
	rows, ok := mc.poses[pose]
	if !ok {
		rows = mc.poses["default"]
	}
	return rows[:]
}

const (
	// mascotWidth is the art box, and mascotTextCol where the text beside
	// it starts: Claude Code's banner puts a 2-column gap after 9 columns.
	mascotWidth   = 9
	mascotTextCol = mascotWidth + 2
	// mascotTop and mascotLeft place the art box in the view: below the
	// blank top row, inside the table's 2-column margin. Clicks use them.
	mascotTop  = 1
	mascotLeft = 2
)

// mascotFrame is one step of a click animation. offset 1 is a crouch: the
// art drops a row, losing its feet, with a puff of poof beside it.
type mascotFrame struct {
	pose   string
	offset int
	poof   string
}

func frames(pose string, offset, n int) []mascotFrame {
	out := make([]mascotFrame, n)
	for i := range out {
		out[i] = mascotFrame{pose: pose, offset: offset}
	}
	return out
}

func crouch() []mascotFrame {
	return []mascotFrame{{pose: "default", offset: 1, poof: "·"}, {pose: "default", offset: 1, poof: "~"}}
}

func concat(parts ...[]mascotFrame) []mascotFrame {
	var out []mascotFrame
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

// Claude Code's two click reactions: a double jump, and a look around.
var (
	jumpAnim = concat(crouch(), frames("arms-up", 0, 3), frames("default", 0, 1),
		crouch(), frames("arms-up", 0, 3), frames("default", 0, 1))
	lookAnim   = concat(frames("look-right", 0, 5), frames("look-left", 0, 5), frames("default", 0, 1))
	clickAnims = [][]mascotFrame{jumpAnim, lookAnim}
)

// mascotPoseFor picks the pose from what the table is doing: eyes to the
// side while it thinks, arms up when it takes a pot.
func mascotPoseFor(thinking, won bool, tick int) string {
	switch {
	case won:
		return "arms-up"
	case thinking:
		if tick/6%2 == 0 {
			return "look-left"
		}
		return "look-right"
	default:
		return "default"
	}
}

// inMascot reports whether a view cell falls on the art box.
func inMascot(x, y int) bool {
	return y >= mascotTop && y < mascotTop+3 && x >= mascotLeft && x < mascotLeft+mascotWidth
}

// render draws the mascot with one text line per art row (title, model,
// stack). extra holds lines (the thinking spinner) that continue the block
// below the art.
func (mc mascot) render(f mascotFrame, title, model, stack string, extra []string) string {
	rows := mc.art(f.pose)
	cells := make([]string, 3)
	if f.offset > 0 {
		// crouch: the art drops a row and loses its feet; poof puffs out at
		// both edges of the body row. Art that fills the box edge to edge
		// (clawd's arms) gets its edges covered, as in Claude Code; inset art
		// (the Codex box) keeps its border and the poof sits just outside.
		body := []rune(rows[1])
		inner := string(body[1 : len(body)-1])
		if body[0] == ' ' {
			inner = string(body[1:])
		}
		p := dimStyle.Render(f.poof)
		cells[1] = mc.style.Render(rows[0])
		cells[2] = p + mc.style.Render(inner) + p
	} else {
		for i, r := range rows {
			cells[i] = mc.style.Render(r)
		}
	}
	right := append([]string{title, model, stack}, extra...)
	margin := strings.Repeat(" ", mascotLeft)
	var b strings.Builder
	for i, line := range right {
		cell := ""
		if i < len(cells) {
			cell = cells[i]
		}
		b.WriteString(margin + padTo(cell, mascotTextCol) + line + "\n")
	}
	return b.String()
}
