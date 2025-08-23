package game

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/MikeLuu99/poker-arena/internal/poker"
	"github.com/MikeLuu99/poker-arena/pkg/models"
)

func (g *Game) postBlinds() {
	activePlayers := g.getActivePlayers()

	if len(activePlayers) < 2 {
		g.addToLog("Not enough active players to post blinds")
		return
	}

	// Find dealer among active players
	dealerIndex := -1
	for i, p := range activePlayers {
		if p.Name == g.State.Players[g.State.DealerPosition].Name {
			dealerIndex = i
			break
		}
	}

	// If current dealer is eliminated, move to next active player
	if dealerIndex == -1 {
		for i := 1; i < len(g.State.Players); i++ {
			nextPos := (g.State.DealerPosition + i) % len(g.State.Players)
			nextPlayer := g.State.Players[nextPos]
			if !contains(g.State.EliminatedPlayers, nextPlayer.Name) {
				g.State.DealerPosition = nextPos
				dealerIndex = 0
				break
			}
		}
	}

	// Assign blinds among active players
	smallBlindIndex := (dealerIndex + 1) % len(activePlayers)
	bigBlindIndex := (dealerIndex + 2) % len(activePlayers)

	smallBlindPlayer := &activePlayers[smallBlindIndex]
	bigBlindPlayer := &activePlayers[bigBlindIndex]

	// Find the actual player structs in gameState to modify
	for i := range g.State.Players {
		if g.State.Players[i].Name == smallBlindPlayer.Name {
			sbAmount := min(g.State.SmallBlind, g.State.Players[i].Chips)
			g.State.Players[i].Chips -= sbAmount
			g.State.Pot += sbAmount
			g.State.PlayerBets[g.State.Players[i].Name] = sbAmount
			g.addToLog(fmt.Sprintf("%s posts small blind $%d", g.State.Players[i].Name, sbAmount))
		}

		if g.State.Players[i].Name == bigBlindPlayer.Name {
			bbAmount := min(g.State.BigBlind, g.State.Players[i].Chips)
			g.State.Players[i].Chips -= bbAmount
			g.State.Pot += bbAmount
			g.State.PlayerBets[g.State.Players[i].Name] = bbAmount
			g.State.CurrentBet = bbAmount
			g.addToLog(fmt.Sprintf("%s posts big blind $%d", g.State.Players[i].Name, bbAmount))
		}
	}

	// Set current player to first active player after big blind
	firstToActIndex := (bigBlindIndex + 1) % len(activePlayers)
	firstToActPlayer := activePlayers[firstToActIndex]

	for i, p := range g.State.Players {
		if p.Name == firstToActPlayer.Name {
			g.State.CurrentPlayer = i
			break
		}
	}
}

func (g *Game) processDecision(decision string, playerIndex int) {
	player := &g.State.Players[playerIndex]
	playerCurrentBet := g.State.PlayerBets[player.Name]
	amountToCall := g.State.CurrentBet - playerCurrentBet

	if strings.HasPrefix(decision, "raise") {
		parts := strings.Fields(decision)
		raiseAmount := g.State.CurrentBet + g.State.MinRaise
		if len(parts) > 1 {
			if amount, err := strconv.Atoi(parts[1]); err == nil {
				raiseAmount = amount
			}
		}

		totalBet := max(raiseAmount, g.State.CurrentBet+g.State.MinRaise)
		actualRaiseAmount := totalBet - playerCurrentBet

		if actualRaiseAmount <= player.Chips {
			g.State.LastRaiseAmount = totalBet - g.State.CurrentBet
			g.State.CurrentBet = totalBet
			g.State.Pot += actualRaiseAmount
			player.Chips -= actualRaiseAmount
			g.State.PlayerBets[player.Name] = totalBet
			g.addToLog(fmt.Sprintf("%s raises to $%d (adding $%d)", player.Name, totalBet, actualRaiseAmount))
		} else {
			// If player can't afford raise, convert to call if possible
			if player.Chips >= amountToCall {
				g.processDecision("call", playerIndex)
			} else {
				g.processDecision("fold", playerIndex)
			}
		}
	} else if decision == "call" {
		if player.Chips >= amountToCall {
			g.State.Pot += amountToCall
			player.Chips -= amountToCall
			g.State.PlayerBets[player.Name] = g.State.CurrentBet
			g.addToLog(fmt.Sprintf("%s calls $%d", player.Name, amountToCall))
		} else {
			g.processDecision("fold", playerIndex)
		}
	} else if decision == "check" {
		if amountToCall == 0 {
			g.addToLog(fmt.Sprintf("%s checks", player.Name))
		} else {
			// Invalid check - convert to call or fold
			if player.Chips >= amountToCall {
				g.processDecision("call", playerIndex)
			} else {
				g.processDecision("fold", playerIndex)
			}
		}
	} else {
		g.addToLog(fmt.Sprintf("%s folds", player.Name))
		g.State.FoldedPlayers = append(g.State.FoldedPlayers, player.Name)
	}
}

func (g *Game) findFirstActivePlayerAfterDealer() int {
	for i := 1; i <= len(g.State.Players); i++ {
		nextPos := (g.State.DealerPosition + i) % len(g.State.Players)
		nextPlayer := g.State.Players[nextPos]
		if !contains(g.State.EliminatedPlayers, nextPlayer.Name) {
			return nextPos
		}
	}
	return g.State.DealerPosition // fallback
}

func (g *Game) advanceRound() {
	switch g.State.Round {
	case "preflop":
		g.State.Round = "flop"
		g.State.CurrentBet = 0
		g.State.PlayerBets = make(map[string]int)
		g.State.LastRaiseAmount = 0
		g.State.BettingComplete = false
		g.State.CurrentPlayer = g.findFirstActivePlayerAfterDealer()

		// Deal flop (pop from end like JavaScript)
		if len(g.State.Deck) >= 3 {
			g.State.CommunityCards = []string{
				g.State.Deck[len(g.State.Deck)-1],
				g.State.Deck[len(g.State.Deck)-2],
				g.State.Deck[len(g.State.Deck)-3],
			}
			g.State.Deck = g.State.Deck[:len(g.State.Deck)-3]
			g.addToLog(fmt.Sprintf("Flop dealt: %s", strings.Join(g.State.CommunityCards, ", ")))
		}

	case "flop":
		g.State.Round = "turn"
		g.State.CurrentBet = 0
		g.State.PlayerBets = make(map[string]int)
		g.State.LastRaiseAmount = 0
		g.State.BettingComplete = false
		g.State.CurrentPlayer = g.findFirstActivePlayerAfterDealer()

		// Deal turn
		if len(g.State.Deck) >= 1 {
			turnCard := g.State.Deck[len(g.State.Deck)-1]
			g.State.Deck = g.State.Deck[:len(g.State.Deck)-1]
			g.State.CommunityCards = append(g.State.CommunityCards, turnCard)
			g.addToLog(fmt.Sprintf("Turn dealt: %s", turnCard))
		}

	case "turn":
		g.State.Round = "river"
		g.State.CurrentBet = 0
		g.State.PlayerBets = make(map[string]int)
		g.State.LastRaiseAmount = 0
		g.State.BettingComplete = false
		g.State.CurrentPlayer = g.findFirstActivePlayerAfterDealer()

		// Deal river
		if len(g.State.Deck) >= 1 {
			riverCard := g.State.Deck[len(g.State.Deck)-1]
			g.State.Deck = g.State.Deck[:len(g.State.Deck)-1]
			g.State.CommunityCards = append(g.State.CommunityCards, riverCard)
			g.addToLog(fmt.Sprintf("River dealt: %s", riverCard))
		}

	case "river":
		g.endHand()
	}
}

func (g *Game) isBettingRoundComplete() bool {
	activePlayers := g.getActivePlayers()

	// Filter out folded players
	var activeUnfoldedPlayers []models.Player
	for _, player := range activePlayers {
		if !contains(g.State.FoldedPlayers, player.Name) {
			activeUnfoldedPlayers = append(activeUnfoldedPlayers, player)
		}
	}

	// All active players have acted and matched the current bet
	for _, player := range activeUnfoldedPlayers {
		playerBet := g.State.PlayerBets[player.Name]
		if playerBet < g.State.CurrentBet && player.Chips > 0 {
			return false // Player hasn't matched the bet and has chips
		}
	}
	return true
}

func (g *Game) endHand() {
	var activePlayers []models.Player
	for _, player := range g.State.Players {
		if !contains(g.State.FoldedPlayers, player.Name) {
			activePlayers = append(activePlayers, player)
		}
	}

	// If only one player remains, they win by default
	if len(activePlayers) == 1 {
		winner := &activePlayers[0]
		// Find the actual player in gameState to update chips
		for i := range g.State.Players {
			if g.State.Players[i].Name == winner.Name {
				g.State.Players[i].Chips += g.State.Pot
				g.addToLog(fmt.Sprintf("%s wins pot of $%d (all others folded)", winner.Name, g.State.Pot))
				break
			}
		}
	} else if len(activePlayers) == 0 {
		// All players folded - award pot to big blind position as a fallback
		bigBlindPos := (g.State.DealerPosition + 2) % len(g.State.Players)
		winner := &g.State.Players[bigBlindPos]
		winner.Chips += g.State.Pot
		g.addToLog(fmt.Sprintf("%s wins pot of $%d (all players folded, awarded to big blind)", winner.Name, g.State.Pot))
	} else {
		// Multiple players remain, compare hands
		// Only compare hands if we have all 5 community cards
		if len(g.State.CommunityCards) < 5 {
			g.addToLog(fmt.Sprintf("Hand ended early with %d community cards - pot split among remaining players", len(g.State.CommunityCards)))
			// Split pot equally among remaining players
			potPerPlayer := g.State.Pot / len(activePlayers)
			remainder := g.State.Pot % len(activePlayers)
			for i, player := range activePlayers {
				for j := range g.State.Players {
					if g.State.Players[j].Name == player.Name {
						share := potPerPlayer
						if i < remainder {
							share++ // Distribute remainder
						}
						g.State.Players[j].Chips += share
						g.addToLog(fmt.Sprintf("%s receives $%d from split pot", player.Name, share))
						break
					}
				}
			}
		} else {
			hands := make([][]string, len(activePlayers))
			for i, player := range activePlayers {
				hands[i] = append(player.Cards, g.State.CommunityCards...)
			}

			winningHands := poker.CompareHands(hands)
			if len(winningHands) > 0 {
				winningHand := winningHands[0]

				// Find the winner by matching the exact hand
				winningCards := strings.Join(winningHand.CardStrings, "")
				for i := range g.State.Players {
					if !contains(g.State.FoldedPlayers, g.State.Players[i].Name) {
						playerCards := strings.Join(append(g.State.Players[i].Cards, g.State.CommunityCards...), "")
						if playerCards == winningCards {
							g.State.Players[i].Chips += g.State.Pot
							g.addToLog(fmt.Sprintf("%s wins pot of $%d with %s", g.State.Players[i].Name, g.State.Pot, winningHand.GetHandName()))
							break
						}
					}
				}
			}
		}
	}

	// Log player balances at end of hand
	balances := make([]string, len(g.State.Players))
	for i, p := range g.State.Players {
		balances[i] = fmt.Sprintf("%s: $%d", p.Name, p.Chips)
	}
	g.addToLog(fmt.Sprintf("Hand #%d complete. Balances: %s", g.State.HandNumber, strings.Join(balances, ", ")))

	// Check for eliminations and tournament end
	g.checkForEliminations()
	g.checkForTournamentEnd()

	// Reset for next hand
	g.State.Pot = 0
	g.State.Round = "preflop"
	g.State.CommunityCards = []string{}
	g.State.FoldedPlayers = []string{}
	g.State.CurrentBet = 0
	g.State.PlayerBets = make(map[string]int)
	g.State.LastRaiseAmount = 0
	g.State.BettingComplete = false

	// Move dealer button to next active player
	remainingPlayers := g.getActivePlayers()
	if len(remainingPlayers) > 1 {
		for i := 1; i < len(g.State.Players); i++ {
			nextPos := (g.State.DealerPosition + i) % len(g.State.Players)
			nextPlayer := g.State.Players[nextPos]
			if !contains(g.State.EliminatedPlayers, nextPlayer.Name) {
				g.State.DealerPosition = nextPos
				break
			}
		}
	}
	g.State.HandNumber++

	// Clear player cards
	for i := range g.State.Players {
		g.State.Players[i].Cards = []string{}
	}
}