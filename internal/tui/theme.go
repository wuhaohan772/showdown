package tui

import (
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// A palette is the full set of colors the table draws with. Every color is
// picked for contrast against its own background — the 2026-09-17 playtest
// on a mid-grey terminal lost the dim text, the black suits, the talk and
// the banner all at once, so nothing here leans on ANSI 8 or bare white.
type palette struct {
	name      string
	dim       lipgloss.Color // dividers, card backs, hints — muted but legible
	text      lipgloss.Color // primary table text
	say       lipgloss.Color // agent table talk
	humanSay  lipgloss.Color // your own echoed lines
	banner    lipgloss.Color // hand results
	redCard   lipgloss.Color // hearts, diamonds
	blackCard lipgloss.Color // spades, clubs
	chip      lipgloss.Color // chip stacks
	shimmer   lipgloss.Color // menu highlight
}

// clawdOrange is Claude Code's own clawd_body color, rgb(215,119,87).
// lipgloss degrades it for terminals that cannot show 24-bit color.
const clawdOrange = lipgloss.Color("#D77757")

// darkPalette targets dark and mid-grey backgrounds: bright text, no ANSI 8.
var darkPalette = palette{
	name:      "dark",
	dim:       lipgloss.Color("250"), // light grey: readable even on mid-grey
	text:      lipgloss.Color("231"), // near-white
	say:       lipgloss.Color("222"), // warm sand
	humanSay:  lipgloss.Color("117"), // pale blue
	banner:    lipgloss.Color("121"), // mint
	redCard:   lipgloss.Color("210"), // salmon: red that survives a grey wash
	blackCard: lipgloss.Color("231"),
	chip:      lipgloss.Color("222"),
	shimmer:   lipgloss.Color("231"),
}

// lightPalette targets light and pale-grey backgrounds: dark ink, no white.
var lightPalette = palette{
	name:      "light",
	dim:       lipgloss.Color("240"),
	text:      lipgloss.Color("16"),
	say:       lipgloss.Color("94"),  // dark amber
	humanSay:  lipgloss.Color("25"),  // navy
	banner:    lipgloss.Color("22"),  // forest
	redCard:   lipgloss.Color("124"), // deep red
	blackCard: lipgloss.Color("16"),
	chip:      lipgloss.Color("136"),
	shimmer:   lipgloss.Color("16"),
}

// ResolveTheme maps a --theme value to a palette name. "auto" (or "") asks
// the terminal; anything unrecognized also falls back to auto rather than
// failing a match over a typo.
func ResolveTheme(arg string) string {
	switch strings.ToLower(strings.TrimSpace(arg)) {
	case "dark":
		return "dark"
	case "light":
		return "light"
	default:
		if lipgloss.HasDarkBackground() {
			return "dark"
		}
		return "light"
	}
}

// ThemeArg reads the theme choice from the flag, falling back to
// SHOWDOWN_THEME so a player can pin it in their shell profile.
func ThemeArg(flagVal string) string {
	if strings.TrimSpace(flagVal) != "" {
		return flagVal
	}
	return os.Getenv("SHOWDOWN_THEME")
}

// ApplyTheme installs a palette into the package styles. Callers pass a
// name from ResolveTheme; the styles are package-level because every
// renderer reaches for them directly.
func ApplyTheme(name string) {
	p := darkPalette
	if name == "light" {
		p = lightPalette
	}
	active = p
	dimStyle = lipgloss.NewStyle().Foreground(p.dim)
	textStyle = lipgloss.NewStyle().Foreground(p.text)
	sayStyle = lipgloss.NewStyle().Foreground(p.say).Italic(true)
	humanSayStyle = lipgloss.NewStyle().Foreground(p.humanSay)
	bannerFresh = lipgloss.NewStyle().Foreground(p.banner).Bold(true)
	bannerSettled = lipgloss.NewStyle().Foreground(p.banner)
	menuDim = lipgloss.NewStyle().Foreground(p.dim)
	menuShimmer = lipgloss.NewStyle().Foreground(p.shimmer).Bold(true)
	redCard = lipgloss.NewStyle().Foreground(p.redCard)
	blackCard = lipgloss.NewStyle().Foreground(p.blackCard)
	cardBack = lipgloss.NewStyle().Foreground(p.dim)
	chipStyle = lipgloss.NewStyle().Foreground(p.chip)
	clawdStyle = lipgloss.NewStyle().Foreground(clawdOrange)
}

var active = darkPalette

func init() { ApplyTheme("dark") }
