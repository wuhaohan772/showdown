package tui

import (
	"strings"
	"testing"

	"github.com/haohanwu/showdown/internal/poker"
)

func card(s string) poker.Card {
	c, err := poker.ParseCard(s)
	if err != nil {
		panic(err)
	}
	return c
}

func stripANSI(s string) string {
	var b strings.Builder
	in := false
	for _, r := range s {
		switch {
		case in:
			if r == 'm' {
				in = false
			}
		case r == '\x1b':
			in = true
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func TestRenderCardFaceUp(t *testing.T) {
	out := stripANSI(RenderCard(card("As"), true))
	lines := strings.Split(out, "\n")
	if len(lines) != 3 {
		t.Fatalf("want 3 lines, got %d: %q", len(lines), out)
	}
	if !strings.Contains(out, "A") || !strings.Contains(out, "♠") {
		t.Errorf("missing rank/suit: %q", out)
	}
	// ten must render as "10" or "T" but keep box width consistent with others
	ten := stripANSI(RenderCard(card("Td"), true))
	if len([]rune(strings.Split(ten, "\n")[0])) != len([]rune(lines[0])) {
		t.Errorf("Td box width differs from As:\n%s\n%s", out, ten)
	}
}

func TestRenderCardFaceDown(t *testing.T) {
	out := stripANSI(RenderCard(card("As"), false))
	if strings.Contains(out, "A") || strings.Contains(out, "♠") {
		t.Errorf("face-down card leaks identity: %q", out)
	}
	if !strings.Contains(out, "?") {
		t.Errorf("want ?? back: %q", out)
	}
}

func TestRenderCardRowRevealed(t *testing.T) {
	cards := []poker.Card{card("As"), card("Kh"), card("7d")}
	out := stripANSI(RenderCardRow(cards, 2))
	if !strings.Contains(out, "A") || !strings.Contains(out, "K") {
		t.Errorf("first two should be face up: %q", out)
	}
	if strings.Contains(out, "7") {
		t.Errorf("third card should be hidden: %q", out)
	}
}
