package poker

import (
	"fmt"
	"math/rand"
	"strings"
)

type Suit int

const (
	Spades Suit = iota
	Hearts
	Diamonds
	Clubs
)

var suitChars = [...]string{"s", "h", "d", "c"}
var rankChars = "23456789TJQKA"

// Card rank is 2..14 (14 = Ace).
type Card struct {
	Rank int
	Suit Suit
}

func (c Card) String() string {
	return string(rankChars[c.Rank-2]) + suitChars[c.Suit]
}

func ParseCard(s string) (Card, error) {
	if len(s) != 2 {
		return Card{}, fmt.Errorf("bad card %q", s)
	}
	r := strings.IndexByte(rankChars, s[0])
	var suit Suit = -1
	for i, sc := range suitChars {
		if sc == string(s[1]) {
			suit = Suit(i)
		}
	}
	if r < 0 || suit < 0 {
		return Card{}, fmt.Errorf("bad card %q", s)
	}
	return Card{Rank: r + 2, Suit: suit}, nil
}

type Deck struct {
	cards []Card
	pos   int
}

func NewDeck(rng *rand.Rand) *Deck {
	d := &Deck{cards: make([]Card, 0, 52)}
	for s := Spades; s <= Clubs; s++ {
		for r := 2; r <= 14; r++ {
			d.cards = append(d.cards, Card{Rank: r, Suit: s})
		}
	}
	rng.Shuffle(52, func(i, j int) { d.cards[i], d.cards[j] = d.cards[j], d.cards[i] })
	return d
}

func (d *Deck) Deal() Card {
	c := d.cards[d.pos]
	d.pos++
	return c
}
