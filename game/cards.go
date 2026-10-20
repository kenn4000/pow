package game

import (
	"fmt"
	"math/rand"
)

type Suit int

const (
	Blade Suit = iota
	Fang
	Shield
)

var suits = [...]string{"Blade", "Fang", "Shield"}
var suitCodes = [...]string{"B", "F", "S"}

func (s Suit) String() string {
	if s < Blade || s > Shield {
		return "Unknown"
	}
	return suits[s]
}

func (s Suit) Code() string {
	if s < Blade || s > Shield {
		return "?"
	}
	return suitCodes[s]
}

func (s Suit) Valid() bool { return s >= Blade && s <= Shield }

func (s Suit) Beats(other Suit) bool {
	return s.Valid() && other.Valid() && (int(s)+1)%len(suits) == int(other)
}

type Card struct {
	Suit Suit `json:"suit"`
	Face int  `json:"face"`
}

func (c Card) Valid() bool { return c.Suit.Valid() && c.Face >= 1 && c.Face <= 9 }

func (c Card) String() string { return fmt.Sprintf("%s %d", c.Suit, c.Face) }

func (c Card) Code() string { return fmt.Sprintf("%s%d", c.Suit.Code(), c.Face) }

func NewDeck() []Card {
	deck := make([]Card, 0, 27)
	for suit := Blade; suit <= Shield; suit++ {
		for face := 1; face <= 9; face++ {
			deck = append(deck, Card{Suit: suit, Face: face})
		}
	}
	return deck
}

func Shuffle(deck []Card, rng *rand.Rand) {
	rng.Shuffle(len(deck), func(i, j int) { deck[i], deck[j] = deck[j], deck[i] })
}

func EffectiveValue(card, opponent Card) int {
	value := card.Face
	if card.Suit.Beats(opponent.Suit) {
		value *= 2
	}
	return value
}
