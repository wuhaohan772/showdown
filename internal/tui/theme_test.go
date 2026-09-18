package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestResolveThemeHonorsExplicitChoice(t *testing.T) {
	if got := ResolveTheme("light"); got != "light" {
		t.Errorf("ResolveTheme(light) = %q", got)
	}
	if got := ResolveTheme("DARK"); got != "dark" {
		t.Errorf("ResolveTheme(DARK) = %q, want case-insensitive match", got)
	}
	// auto and typos both defer to the terminal; either answer is valid, but
	// it must be one of the two palettes rather than an empty string.
	for _, arg := range []string{"", "auto", "purple"} {
		if got := ResolveTheme(arg); got != "dark" && got != "light" {
			t.Errorf("ResolveTheme(%q) = %q, want dark or light", arg, got)
		}
	}
}

func TestThemeArgPrefersFlagThenEnv(t *testing.T) {
	t.Setenv("SHOWDOWN_THEME", "light")
	if got := ThemeArg("dark"); got != "dark" {
		t.Errorf("flag must win: got %q", got)
	}
	if got := ThemeArg(""); got != "light" {
		t.Errorf("env fallback = %q", got)
	}
	if got := ThemeArg("   "); got != "light" {
		t.Errorf("whitespace flag must fall back to env: got %q", got)
	}
}

// The 2026-09-17 playtest lost every dim element on a mid-grey terminal:
// ANSI 8 and bare white both washed out. No palette may use them.
func TestPalettesAvoidWashedOutColors(t *testing.T) {
	for _, p := range []palette{darkPalette, lightPalette} {
		for name, c := range map[string]lipgloss.Color{
			"dim": p.dim, "text": p.text, "say": p.say, "humanSay": p.humanSay,
			"banner": p.banner, "redCard": p.redCard, "blackCard": p.blackCard,
			"chip": p.chip, "shimmer": p.shimmer,
		} {
			switch string(c) {
			case "8", "15", "7":
				t.Errorf("%s palette: %s uses washed-out ANSI %s", p.name, name, c)
			case "":
				t.Errorf("%s palette: %s is unset", p.name, name)
			}
		}
	}
}

func TestApplyThemeSwapsStyles(t *testing.T) {
	ApplyTheme("light")
	lightInk := blackCard.GetForeground()
	ApplyTheme("dark")
	darkInk := blackCard.GetForeground()
	if lightInk == darkInk {
		t.Error("black-suit ink must differ between the light and dark palettes")
	}
	if !strings.HasPrefix(string(clawdOrange), "#") {
		t.Errorf("clawd colour = %q, want Claude Code's rgb(215,119,87) hex", clawdOrange)
	}
}
