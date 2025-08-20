package poker

import (
	"math/rand"
	"time"
)

func InitializeDeck() []string {
	suits := []string{"♠", "♣", "♥", "♦"}
	values := []string{"2", "3", "4", "5", "6", "7", "8", "9", "10", "J", "Q", "K", "A"}
	var deck []string

	for _, suit := range suits {
		for _, value := range values {
			deck = append(deck, value+suit)
		}
	}

	return Shuffle(deck)
}

func Shuffle(array []string) []string {
	rand.Seed(time.Now().UnixNano())
	for i := len(array) - 1; i > 0; i-- {
		j := rand.Intn(i + 1)
		array[i], array[j] = array[j], array[i]
	}
	return array
}