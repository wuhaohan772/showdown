package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/haohanwu/showdown/internal/poker"
)

var (
	redCard   = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	blackCard = lipgloss.NewStyle().Foreground(lipgloss.Color("15"))
	cardBack  = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
)

func SuitGlyph(s poker.Suit) string {
	return [...]string{"♠", "♥", "♦", "♣"}[s]
}

func rankLabel(r int) string {
	return map[int]string{10: "T", 11: "J", 12: "Q", 13: "K", 14: "A"}[r] + ""
}

// RenderCard draws a 3-line card box. Interior is 2 runes wide.
func RenderCard(c poker.Card, faceUp bool) string {
	if !faceUp {
		return cardBack.Render("┌──┐\n│??│\n└──┘")
	}
	label := rankLabel(c.Rank)
	if label == "" {
		label = string(rune('0' + c.Rank))
	}
	style := blackCard
	if c.Suit == poker.Hearts || c.Suit == poker.Diamonds {
		style = redCard
	}
	return style.Render("┌──┐\n│" + label + SuitGlyph(c.Suit) + "│\n└──┘")
}

// RenderCardRow joins cards horizontally; indexes >= revealed are face down.
func RenderCardRow(cards []poker.Card, revealed int) string {
	if len(cards) == 0 {
		return ""
	}
	blocks := make([]string, len(cards))
	for i, c := range cards {
		blocks[i] = RenderCard(c, i < revealed)
	}
	rows := make([]string, 3)
	for _, b := range blocks {
		for i, line := range strings.Split(b, "\n") {
			if rows[i] != "" {
				rows[i] += " "
			}
			rows[i] += line
		}
	}
	return strings.Join(rows, "\n")
}
