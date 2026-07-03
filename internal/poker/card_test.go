package poker

import (
	"math/rand"
	"testing"
)

func TestCardString(t *testing.T) {
	cases := []struct {
		c    Card
		want string
	}{
		{Card{14, Spades}, "As"},
		{Card{10, Diamonds}, "Td"},
		{Card{7, Clubs}, "7c"},
		{Card{13, Hearts}, "Kh"},
		{Card{2, Spades}, "2s"},
	}
	for _, tc := range cases {
		if got := tc.c.String(); got != tc.want {
			t.Errorf("String() = %q, want %q", got, tc.want)
		}
	}
}

func TestParseCardRoundTrip(t *testing.T) {
	for _, s := range []string{"As", "Td", "7c", "Kh", "2s", "Qd", "Jh", "9c"} {
		c, err := ParseCard(s)
		if err != nil {
			t.Fatalf("ParseCard(%q): %v", s, err)
		}
		if c.String() != s {
			t.Errorf("round trip %q -> %q", s, c.String())
		}
	}
	if _, err := ParseCard("Xx"); err == nil {
		t.Error("ParseCard(Xx) should fail")
	}
}

func TestDeckDeals52Unique(t *testing.T) {
	d := NewDeck(rand.New(rand.NewSource(1)))
	seen := map[string]bool{}
	for i := 0; i < 52; i++ {
		c := d.Deal()
		if seen[c.String()] {
			t.Fatalf("duplicate card %s", c)
		}
		seen[c.String()] = true
	}
}

func TestDeckShuffled(t *testing.T) {
	a := NewDeck(rand.New(rand.NewSource(1)))
	b := NewDeck(rand.New(rand.NewSource(2)))
	same := true
	for i := 0; i < 10; i++ {
		if a.Deal() != b.Deal() {
			same = false
		}
	}
	if same {
		t.Error("two differently-seeded decks dealt identical first 10 cards")
	}
}
