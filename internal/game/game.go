package game

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/MikeLuu99/poker-arena/internal/ai"
	"github.com/MikeLuu99/poker-arena/internal/poker"
	"github.com/MikeLuu99/poker-arena/pkg/models"
)

type Game struct {
	ID        int
	State     *models.GameState
	stopChan  chan bool
	result    *models.GameResult
	startTime time.Time
}

var models_list = []string{
	"google/gemini-2.5-flash",
	"openai/gpt-5-nano",
	"openai/gpt-oss-120b",
	"anthropic/claude-3.5-haiku",
}

var initialTotalChips *int

func NewGame() *Game {
	return NewGameWithID(1)
}

func NewGameWithID(gameID int) *Game {
	players := make([]models.Player, len(models_list))
	for i, model := range models_list {
		players[i] = models.Player{
			Name:  model,
			Chips: 20,
			Cards: []string{},
			Model: model,
		}
	}

	gameState := &models.GameState{
		Players:           players,
		Deck:              []string{},
		CommunityCards:    []string{},
		Pot:               0,
		CurrentPlayer:     0,
		Round:             "preflop",
		HandNumber:        1,
		GameLog:           []string{},
		CurrentBet:        0,
		PlayerBets:        make(map[string]int),
		LastRaiseAmount:   0,
		MinRaise:          10,
		FoldedPlayers:     []string{},
		DealerPosition:    0,
		SmallBlind:        5,
		BigBlind:          10,
		BettingComplete:   false,
		EliminatedPlayers: []string{},
		GameEnded:         false,
	}

	return &Game{
		ID:        gameID,
		State:     gameState,
		stopChan:  make(chan bool),
		result:    nil,
		startTime: time.Now(),
	}
}

func (g *Game) Start() *models.GameResult {
	for !g.State.GameEnded {
		select {
		case <-g.stopChan:
			return nil
		default:
			g.advanceGame()
			time.Sleep(2 * time.Second)
		}
	}
	log.Println("🏆 Tournament has ended! Game loop stopped.")
	return g.result
}

func (g *Game) Stop() {
	close(g.stopChan)
}

func (g *Game) GetResult() *models.GameResult {
	return g.result
}

func (g *Game) GetStartTime() time.Time {
	return g.startTime
}

func (g *Game) addToLog(message string) {
	timestamp := time.Now().Format("15:04:05")
	logMessage := fmt.Sprintf("[%s] %s", timestamp, message)

	// Add to beginning of slice
	g.State.GameLog = append([]string{logMessage}, g.State.GameLog...)

	// Keep only last 50 messages
	if len(g.State.GameLog) > 50 {
		g.State.GameLog = g.State.GameLog[:50]
	}

	// Also print to console for debugging
	log.Printf("GAME: %s", message)
}

func (g *Game) checkChipConservation() bool {
	totalPlayerChips := 0
	for _, player := range g.State.Players {
		totalPlayerChips += player.Chips
	}
	totalChips := totalPlayerChips + g.State.Pot

	// Initialize the expected chips on first call
	if initialTotalChips == nil {
		initialTotalChips = &totalChips
	}

	if totalChips != *initialTotalChips {
		log.Printf("🚨 CHIP LEAK DETECTED! Expected: %d, Actual: %d", *initialTotalChips, totalChips)
		log.Printf("Player chips: %d, Pot: %d", totalPlayerChips, g.State.Pot)

		balances := make([]string, len(g.State.Players))
		for i, p := range g.State.Players {
			balances[i] = fmt.Sprintf("%s: %d", p.Name, p.Chips)
		}
		log.Printf("Player balances: %s", strings.Join(balances, ", "))
	}
	return totalChips == *initialTotalChips
}

func (g *Game) checkForEliminations() []string {
	var newlyEliminated []string

	for _, player := range g.State.Players {
		if player.Chips == 0 && !contains(g.State.EliminatedPlayers, player.Name) {
			g.State.EliminatedPlayers = append(g.State.EliminatedPlayers, player.Name)
			newlyEliminated = append(newlyEliminated, player.Name)
			g.addToLog(fmt.Sprintf("%s has been eliminated from the tournament!", player.Name))
		}
	}

	return newlyEliminated
}

func (g *Game) getActivePlayers() []models.Player {
	var active []models.Player
	for _, player := range g.State.Players {
		if !contains(g.State.EliminatedPlayers, player.Name) {
			active = append(active, player)
		}
	}
	return active
}

func (g *Game) checkForTournamentEnd() bool {
	activePlayers := g.getActivePlayers()

	if len(activePlayers) == 1 {
		g.State.GameEnded = true
		winner := activePlayers[0]
		
		// Create game result
		duration := time.Since(g.startTime)
		g.result = &models.GameResult{
			GameID:       g.ID,
			Winner:       winner,
			TotalHands:   g.State.HandNumber,
			AllPlayers:   g.State.Players,
			Eliminated:   g.State.EliminatedPlayers,
			FinalChips:   winner.Chips,
			GameDuration: duration.String(),
			StartTime:    g.startTime,
			EndTime:      time.Now(),
		}
		
		g.addToLog(fmt.Sprintf("🏆 TOURNAMENT WINNER: %s wins with $%d! 🏆", winner.Name, winner.Chips))
		log.Printf("🏆 Tournament ended! Winner: %s with $%d in %d hands (Duration: %v)", 
			winner.Name, winner.Chips, g.State.HandNumber, duration)
		return true
	}

	return false
}

func (g *Game) advanceGame() {
	// Check if tournament has ended
	if g.State.GameEnded {
		return
	}

	// Deal new hand
	if g.State.Round == "preflop" &&
		len(g.State.CommunityCards) == 0 &&
		len(g.State.PlayerBets) == 0 {

		// Check if we have enough active players
		activePlayers := g.getActivePlayers()
		if len(activePlayers) < 2 {
			g.checkForTournamentEnd()
			return
		}

		// Initialize new hand
		g.State.Deck = poker.InitializeDeck()
		g.State.CurrentBet = 0
		g.State.PlayerBets = make(map[string]int)
		g.State.FoldedPlayers = []string{}
		g.State.BettingComplete = false

		// Deal cards only to active players
		for i := range g.State.Players {
			if !contains(g.State.EliminatedPlayers, g.State.Players[i].Name) {
				if len(g.State.Deck) >= 2 {
					g.State.Players[i].Cards = []string{
						g.State.Deck[len(g.State.Deck)-1],
						g.State.Deck[len(g.State.Deck)-2],
					}
					g.State.Deck = g.State.Deck[:len(g.State.Deck)-2]
				}
			}
		}

		// Post blinds
		g.postBlinds()
		g.checkChipConservation()
		g.addToLog(fmt.Sprintf("Hand #%d begins. Dealer: %s", g.State.HandNumber, g.State.Players[g.State.DealerPosition].Name))
	}

	currentPlayer := g.State.Players[g.State.CurrentPlayer]

	// Get current player's decision (skip if eliminated)
	if !contains(g.State.FoldedPlayers, currentPlayer.Name) &&
		!contains(g.State.EliminatedPlayers, currentPlayer.Name) {

		decision, err := ai.GetAIDecision(currentPlayer, g.State)
		if err != nil {
			log.Printf("Error getting AI decision: %v", err)
			decision = "fold"
		}
		g.processDecision(decision, g.State.CurrentPlayer)
		g.checkChipConservation()
	}

	// If all players have folded except one, end the hand
	activePlayers := g.getActivePlayers()
	activeUnfoldedPlayers := 0
	for _, player := range activePlayers {
		if !contains(g.State.FoldedPlayers, player.Name) {
			activeUnfoldedPlayers++
		}
	}
	if activeUnfoldedPlayers <= 1 {
		g.endHand()
		g.checkChipConservation()
		return
	}

	// Move to next player
	g.State.CurrentPlayer = (g.State.CurrentPlayer + 1) % len(g.State.Players)

	// Check if betting round is complete
	if g.isBettingRoundComplete() {
		g.advanceRound()
	}
}

// Helper functions
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
