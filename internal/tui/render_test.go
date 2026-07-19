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

func TestRenderCardRowDealSlidingCard(t *testing.T) {
	cards := []poker.Card{{Rank: 14, Suit: poker.Spades}, {Rank: 13, Suit: poker.Hearts}, {Rank: 7, Suit: poker.Diamonds}}
	got := RenderCardRowDeal(cards, 1, 1, 4)
	lines := strings.Split(got, "\n")
	if len(lines) != 3 {
		t.Fatalf("want 3 lines, got %d: %q", len(lines), got)
	}
	// Landed card is face up (A♠ visible), sliding card is a face-down back
	// after 4 spaces of gap, third card absent.
	if !strings.Contains(got, "A♠") {
		t.Errorf("landed card must render face up: %q", got)
	}
	if !strings.Contains(got, "??") {
		t.Errorf("sliding card must render face down: %q", got)
	}
	if strings.Contains(got, "7♦") || strings.Contains(got, "K♥") {
		t.Errorf("undealt/sliding cards must not show faces: %q", got)
	}
	// gap: landed card box (4 wide) + separating space + 4 offset spaces before the back
	if !strings.Contains(lines[0], "┐     ┌") {
		t.Errorf("want 5-space gap (1 join + 4 offset) before sliding card, got %q", lines[0])
	}
}

func TestRenderCardRowDealNoIncoming(t *testing.T) {
	cards := []poker.Card{{Rank: 14, Suit: poker.Spades}, {Rank: 13, Suit: poker.Hearts}}
	got := RenderCardRowDeal(cards, 2, 1, -1) // offset -1: no card currently sliding into this row
	if strings.Contains(got, "??") {
		t.Errorf("offset -1 must not draw an incoming card: %q", got)
	}
	if !strings.Contains(got, "A♠") || strings.Contains(got, "K♥") {
		t.Errorf("exactly the landed cards must render: %q", got)
	}
}

func TestRenderCardRowDealComplete(t *testing.T) {
	cards := []poker.Card{{Rank: 14, Suit: poker.Spades}, {Rank: 13, Suit: poker.Hearts}}
	if got, want := RenderCardRowDeal(cards, 2, 2, -1), RenderCardRow(cards, 2); got != want {
		t.Errorf("fully landed deal row must equal RenderCardRow:\n%q\n%q", got, want)
	}
}

func TestRenderChips(t *testing.T) {
	if got := renderChips(0, 20); got != "" {
		t.Errorf("empty pot: want empty string, got %q", got)
	}
	if got := renderChips(60, 20); strings.Count(got, "●") != 3 {
		t.Errorf("60/20 pot: want 3 chips, got %q", got)
	}
	if got := renderChips(10000, 20); strings.Count(got, "●") != 12 {
		t.Errorf("huge pot: want cap of 12 chips, got %q", got)
	}
	if got := renderChips(100, 0); got != "" {
		t.Errorf("bb 0 must not divide by zero, want empty, got %q", got)
	}
}
