package poker

import (
	"sort"
)

// Card values mapping
var VALUES = map[string]int{
	"2": 2, "3": 3, "4": 4, "5": 5, "6": 6, "7": 7, "8": 8,
	"9": 9, "10": 10, "J": 11, "Q": 12, "K": 13, "A": 14,
}

// Hand rankings from highest to lowest
const (
	ROYAL_FLUSH = iota + 1
	STRAIGHT_FLUSH
	FOUR_OF_A_KIND
	FULL_HOUSE
	FLUSH
	STRAIGHT
	THREE_OF_A_KIND
	TWO_PAIR
	ONE_PAIR
	HIGH_CARD
)

// Reverse the order to match JavaScript logic (higher number = better hand)
var HAND_RANKINGS = map[string]int{
	"ROYAL_FLUSH":     10,
	"STRAIGHT_FLUSH":  9,
	"FOUR_OF_A_KIND":  8,
	"FULL_HOUSE":      7,
	"FLUSH":           6,
	"STRAIGHT":        5,
	"THREE_OF_A_KIND": 4,
	"TWO_PAIR":        3,
	"ONE_PAIR":        2,
	"HIGH_CARD":       1,
}

type Card struct {
	Value string
	Suit  string
}

type HandScore struct {
	Rank  int
	Value []int
}

type PokerHand struct {
	CardStrings  []string
	Cards        []Card
	SortedValues []int
	Suits        []string
	ValueCounts  map[int]int
	Score        HandScore
}

func parseCard(cardString string) Card {
	// Handle 10 as special case
	if len(cardString) == 3 {
		return Card{
			Value: cardString[:2],
			Suit:  string(cardString[2]),
		}
	}
	return Card{
		Value: string(cardString[0]),
		Suit:  string(cardString[1]),
	}
}

func NewPokerHand(cardStrings []string) *PokerHand {
	// Validate that we have at least 5 cards for proper poker evaluation
	if len(cardStrings) < 5 {
		// For now, just pad with high cards to avoid crashes
		// In a real game, this shouldn't happen
		paddedCards := make([]string, len(cardStrings))
		copy(paddedCards, cardStrings)
		for len(paddedCards) < 5 {
			paddedCards = append(paddedCards, "2♠") // Add low cards as padding
		}
		cardStrings = paddedCards
	}

	ph := &PokerHand{
		CardStrings: cardStrings,
		Cards:       make([]Card, len(cardStrings)),
		Suits:       make([]string, len(cardStrings)),
		ValueCounts: make(map[int]int),
	}

	// Parse cards
	for i, cardStr := range cardStrings {
		ph.Cards[i] = parseCard(cardStr)
	}

	// Get sorted values
	ph.SortedValues = make([]int, len(ph.Cards))
	for i, card := range ph.Cards {
		ph.SortedValues[i] = VALUES[card.Value]
	}
	sort.Slice(ph.SortedValues, func(i, j int) bool {
		return ph.SortedValues[i] > ph.SortedValues[j]
	})

	// Get suits
	for i, card := range ph.Cards {
		ph.Suits[i] = card.Suit
	}

	// Get value counts
	ph.ValueCounts = ph.getValueCounts()

	// Evaluate hand
	ph.Score = ph.evaluateHand()

	return ph
}

func (ph *PokerHand) getValueCounts() map[int]int {
	counts := make(map[int]int)
	for _, card := range ph.Cards {
		value := VALUES[card.Value]
		counts[value]++
	}
	return counts
}

func (ph *PokerHand) hasFlush() bool {
	if len(ph.Suits) == 0 {
		return false
	}
	firstSuit := ph.Suits[0]
	for _, suit := range ph.Suits {
		if suit != firstSuit {
			return false
		}
	}
	return true
}

func (ph *PokerHand) hasStraight() bool {
	// Create unique values and sort them
	uniqueValues := make(map[int]bool)
	for _, value := range ph.SortedValues {
		uniqueValues[value] = true
	}

	values := make([]int, 0, len(uniqueValues))
	for value := range uniqueValues {
		values = append(values, value)
	}
	sort.Slice(values, func(i, j int) bool {
		return values[i] > values[j]
	})

	// Need at least 5 unique values for a straight
	if len(values) < 5 {
		return false
	}

	// Handle Ace-low straight (A,2,3,4,5)
	if len(values) > 0 && values[0] == 14 && len(values) > 1 && values[1] == 5 {
		// Remove ace from front and add as 1 at the end
		values = values[1:]
		values = append(values, 1)
	}

	// Check if we have any 5-card consecutive sequence
	for start := 0; start <= len(values)-5; start++ {
		isConsecutive := true
		for i := 0; i < 4; i++ {
			if values[start+i]-values[start+i+1] != 1 {
				isConsecutive = false
				break
			}
		}
		if isConsecutive {
			return true
		}
	}
	return false
}

func (ph *PokerHand) evaluateHand() HandScore {
	// Get sorted counts
	counts := make([]int, 0, len(ph.ValueCounts))
	for _, count := range ph.ValueCounts {
		counts = append(counts, count)
	}
	sort.Slice(counts, func(i, j int) bool {
		return counts[i] > counts[j]
	})

	isFlush := ph.hasFlush()
	isStraight := ph.hasStraight()

	// Royal Flush (need at least 5 cards)
	if isFlush && isStraight && len(ph.SortedValues) >= 5 && ph.SortedValues[0] == 14 && ph.SortedValues[4] == 10 {
		return HandScore{Rank: HAND_RANKINGS["ROYAL_FLUSH"], Value: ph.SortedValues}
	}

	// Straight Flush
	if isFlush && isStraight {
		return HandScore{Rank: HAND_RANKINGS["STRAIGHT_FLUSH"], Value: ph.SortedValues}
	}

	// Four of a Kind
	if len(counts) > 0 && counts[0] == 4 {
		var quadValue int
		for value, count := range ph.ValueCounts {
			if count == 4 {
				quadValue = value
				break
			}
		}
		var kicker int
		for _, value := range ph.SortedValues {
			if value != quadValue {
				kicker = value
				break
			}
		}
		return HandScore{Rank: HAND_RANKINGS["FOUR_OF_A_KIND"], Value: []int{quadValue, kicker}}
	}

	// Full House
	if len(counts) >= 2 && counts[0] == 3 && counts[1] == 2 {
		var tripValue, pairValue int
		for value, count := range ph.ValueCounts {
			if count == 3 {
				tripValue = value
			} else if count == 2 {
				pairValue = value
			}
		}
		return HandScore{Rank: HAND_RANKINGS["FULL_HOUSE"], Value: []int{tripValue, pairValue}}
	}

	// Flush
	if isFlush {
		return HandScore{Rank: HAND_RANKINGS["FLUSH"], Value: ph.SortedValues}
	}

	// Straight
	if isStraight {
		return HandScore{Rank: HAND_RANKINGS["STRAIGHT"], Value: ph.SortedValues}
	}

	// Three of a Kind
	if len(counts) > 0 && counts[0] == 3 {
		var tripValue int
		for value, count := range ph.ValueCounts {
			if count == 3 {
				tripValue = value
				break
			}
		}
		kickers := make([]int, 0)
		for _, value := range ph.SortedValues {
			if value != tripValue {
				kickers = append(kickers, value)
			}
		}
		result := []int{tripValue}
		result = append(result, kickers...)
		return HandScore{Rank: HAND_RANKINGS["THREE_OF_A_KIND"], Value: result}
	}

	// Two Pair
	if len(counts) >= 2 && counts[0] == 2 && counts[1] == 2 {
		pairs := make([]int, 0)
		for value, count := range ph.ValueCounts {
			if count == 2 {
				pairs = append(pairs, value)
			}
		}
		sort.Slice(pairs, func(i, j int) bool {
			return pairs[i] > pairs[j]
		})

		var kicker int
		for _, value := range ph.SortedValues {
			found := false
			for _, pair := range pairs {
				if value == pair {
					found = true
					break
				}
			}
			if !found {
				kicker = value
				break
			}
		}
		result := append(pairs, kicker)
		return HandScore{Rank: HAND_RANKINGS["TWO_PAIR"], Value: result}
	}

	// One Pair
	if len(counts) > 0 && counts[0] == 2 {
		var pairValue int
		for value, count := range ph.ValueCounts {
			if count == 2 {
				pairValue = value
				break
			}
		}
		kickers := make([]int, 0)
		for _, value := range ph.SortedValues {
			if value != pairValue {
				kickers = append(kickers, value)
			}
		}
		result := []int{pairValue}
		result = append(result, kickers...)
		return HandScore{Rank: HAND_RANKINGS["ONE_PAIR"], Value: result}
	}

	// High Card
	return HandScore{Rank: HAND_RANKINGS["HIGH_CARD"], Value: ph.SortedValues}
}

func (ph *PokerHand) GetHandName() string {
	rankNames := map[int]string{
		10: "Royal Flush",
		9:  "Straight Flush",
		8:  "Four of a Kind",
		7:  "Full House",
		6:  "Flush",
		5:  "Straight",
		4:  "Three of a Kind",
		3:  "Two Pair",
		2:  "One Pair",
		1:  "High Card",
	}
	return rankNames[ph.Score.Rank]
}

func CompareHands(hands [][]string) []*PokerHand {
	evaluatedHands := make([]*PokerHand, len(hands))
	for i, hand := range hands {
		evaluatedHands[i] = NewPokerHand(hand)
	}

	// Sort hands by rank first, then by value arrays
	sort.Slice(evaluatedHands, func(i, j int) bool {
		a, b := evaluatedHands[i], evaluatedHands[j]
		if a.Score.Rank != b.Score.Rank {
			return b.Score.Rank < a.Score.Rank // Higher rank wins
		}

		// Compare value arrays element by element
		minLen := len(a.Score.Value)
		if len(b.Score.Value) < minLen {
			minLen = len(b.Score.Value)
		}

		for k := 0; k < minLen; k++ {
			if a.Score.Value[k] != b.Score.Value[k] {
				return b.Score.Value[k] < a.Score.Value[k] // Higher value wins
			}
		}

		return false // Hands are equal
	})

	return evaluatedHands
}