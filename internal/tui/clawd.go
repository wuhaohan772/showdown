package tui

import "strings"

// Clawd, the Claude Code mascot, sits on the opponent's side of the table
// when the agent is Claude Code. The glyph rows and the clawd_body color
// are Claude Code's own (v2.1.274, poses default/look-left/look-right/
// arms-up); only the pose-picking is ours.
type clawdPose struct {
	r1L, r1E, r1R string
	r2L, r2R      string
}

var clawdPoses = map[string]clawdPose{
	"default":    {r1L: " ▐", r1E: "▛███▛█", r1R: "", r2L: "▝▜", r2R: "█▀"},
	"look-left":  {r1L: " ▐", r1E: "▟███▟█", r1R: "", r2L: "▝▜", r2R: "█▀"},
	"look-right": {r1L: " ▐", r1E: "█▟███▟", r1R: "", r2L: "▝▜", r2R: "█▀"},
	"arms-up":    {r1L: "▗▟", r1E: "▛███▛█", r1R: "▄", r2L: " ▜", r2R: "█▘"},
}

// clawdArt returns the three glyph rows for a pose, unstyled. An unknown
// pose falls back to default rather than drawing nothing.
func clawdArt(pose string) []string {
	p, ok := clawdPoses[pose]
	if !ok {
		p = clawdPoses["default"]
	}
	return []string{
		p.r1L + p.r1E + p.r1R,
		p.r2L + "█████" + p.r2R,
		"  ▝▝ ▝▝  ",
	}
}

// clawdPoseFor picks the pose from what the table is doing: eyes to the
// side while it thinks, arms up when it takes a pot.
func clawdPoseFor(thinking, won bool, tick int) string {
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

// renderClawd draws the mascot with the opponent's name and stack to its
// right, vertically centered on the art. side holds any extra lines (the
// thinking spinner) that belong in the same block.
func renderClawd(pose, name, stack string, extra []string) string {
	art := clawdArt(pose)
	right := []string{"", name, stack}
	right = append(right, extra...)
	for len(right) < len(art) {
		right = append(right, "")
	}
	var b strings.Builder
	for i, line := range right {
		if i < len(art) {
			b.WriteString("  " + clawdStyle.Render(padTo(art[i], 10)))
		} else {
			b.WriteString("  " + strings.Repeat(" ", 10))
		}
		b.WriteString(line + "\n")
	}
	return b.String()
}
