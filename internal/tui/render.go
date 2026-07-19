package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/wuhaohan772/showdown/internal/poker"
)

var (
	redCard   = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	blackCard = lipgloss.NewStyle().Foreground(lipgloss.Color("15"))
	cardBack  = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	chipStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
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

// RenderCardRowDeal draws a row mid-deal: the first landed cards (face up
// where i < revealed), then — if a card is currently sliding into this row
// (landed < len(cards) && offset >= 0) — one face-down card after offset
// spaces. Cards beyond the sliding one are not drawn at all.
func RenderCardRowDeal(cards []poker.Card, revealed, landed, offset int) string {
	if landed > len(cards) {
		landed = len(cards)
	}
	blocks := make([]string, 0, landed+1)
	for i := 0; i < landed; i++ {
		blocks = append(blocks, RenderCard(cards[i], i < revealed))
	}
	if landed < len(cards) && offset >= 0 {
		pad := strings.Repeat(" ", offset)
		b := strings.Split(RenderCard(cards[landed], false), "\n")
		for i := range b {
			b[i] = pad + b[i]
		}
		blocks = append(blocks, strings.Join(b, "\n"))
	}
	if len(blocks) == 0 {
		return ""
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

// renderChips draws the pot as a pile of chips, one per big blind, capped so
// the line never crowds the pot number.
func renderChips(pot, bb int) string {
	if bb <= 0 || pot <= 0 {
		return ""
	}
	n := pot / bb
	if n < 1 {
		n = 1
	}
	if n > 12 {
		n = 12
	}
	return chipStyle.Render(strings.Repeat("●", n))
}
