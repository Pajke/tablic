package game

import (
	"fmt"
	"math/rand"
)

var allSuits = []Suit{Clubs, Diamonds, Hearts, Spades}
var allRanks = []Rank{Rank2, Rank3, Rank4, Rank5, Rank6, Rank7, Rank8, Rank9, Rank10, RankJ, RankQ, RankK, RankA}

// NewDeck returns a fresh ordered 52-card deck.
func NewDeck() []Card {
	cards := make([]Card, 0, 52)
	for _, suit := range allSuits {
		for _, rank := range allRanks {
			cards = append(cards, Card{
				ID:   fmt.Sprintf("%s-%s", rank, suit),
				Rank: rank,
				Suit: suit,
			})
		}
	}
	return cards
}

// Shuffle performs a Fisher-Yates shuffle on the deck in place.
func Shuffle(deck []Card) {
	for i := len(deck) - 1; i > 0; i-- {
		j := rand.Intn(i + 1)
		deck[i], deck[j] = deck[j], deck[i]
	}
}
