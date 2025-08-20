package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
)

// OpenRouter API configuration
var (
	OPENROUTER_API_KEY  string
	OPENROUTER_BASE_URL = "https://openrouter.ai/api/v1"
	rateLimited         = false
)

var models = []string{
	"google/gemini-2.5-flash",
	"openai/gpt-5-nano",
	"openai/gpt-oss-120b",
	"anthropic/claude-3.5-haiku",
}

type Player struct {
	Name  string   `json:"name"`
	Chips int      `json:"chips"`
	Cards []string `json:"cards"`
	Model string   `json:"model"`
}

type GameState struct {
	Players           []Player       `json:"players"`
	Deck              []string       `json:"deck"`
	CommunityCards    []string       `json:"communityCards"`
	Pot               int            `json:"pot"`
	CurrentPlayer     int            `json:"currentPlayer"`
	Round             string         `json:"round"`
	HandNumber        int            `json:"handNumber"`
	GameLog           []string       `json:"gameLog"`
	CurrentBet        int            `json:"currentBet"`
	PlayerBets        map[string]int `json:"playerBets"`
	LastRaiseAmount   int            `json:"lastRaiseAmount"`
	MinRaise          int            `json:"minRaise"`
	FoldedPlayers     []string       `json:"foldedPlayers"`
	DealerPosition    int            `json:"dealerPosition"`
	SmallBlind        int            `json:"smallBlind"`
	BigBlind          int            `json:"bigBlind"`
	BettingComplete   bool           `json:"bettingComplete"`
	EliminatedPlayers []string       `json:"eliminatedPlayers"`
	GameEnded         bool           `json:"gameEnded"`
}

var gameState = GameState{
	Players: func() []Player {
		players := make([]Player, len(models))
		for i, model := range models {
			players[i] = Player{
				Name:  model,
				Chips: 100,
				Cards: []string{},
				Model: model,
			}
		}
		return players
	}(),
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

var initialTotalChips *int
var clients = make(map[*websocket.Conn]bool)
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins in development
	},
}

type PokerActionTool struct {
	Type     string `json:"type"`
	Function struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Parameters  struct {
			Type       string `json:"type"`
			Properties struct {
				Action struct {
					Type        string   `json:"type"`
					Enum        []string `json:"enum"`
					Description string   `json:"description"`
				} `json:"action"`
				RaiseAmount struct {
					Type        string `json:"type"`
					Description string `json:"description"`
					Minimum     int    `json:"minimum"`
				} `json:"raise_amount"`
				Reasoning struct {
					Type        string `json:"type"`
					Description string `json:"description"`
				} `json:"reasoning"`
			} `json:"properties"`
			Required []string `json:"required"`
		} `json:"parameters"`
	} `json:"function"`
}

type OpenRouterRequest struct {
	Model    string `json:"model"`
	Messages []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
	Tools      []PokerActionTool `json:"tools,omitempty"`
	ToolChoice interface{}       `json:"tool_choice,omitempty"`
}

type OpenRouterResponse struct {
	Choices []struct {
		Message struct {
			Content   string `json:"content"`
			ToolCalls []struct {
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"message"`
	} `json:"choices"`
}

type ActionArgs struct {
	Action      string `json:"action"`
	RaiseAmount int    `json:"raise_amount"`
	Reasoning   string `json:"reasoning"`
}

func addToLog(message string) {
	timestamp := time.Now().Format("15:04:05")
	logMessage := fmt.Sprintf("[%s] %s", timestamp, message)

	// Add to beginning of slice
	gameState.GameLog = append([]string{logMessage}, gameState.GameLog...)

	// Keep only last 50 messages
	if len(gameState.GameLog) > 50 {
		gameState.GameLog = gameState.GameLog[:50]
	}

	// Also print to console for debugging
	log.Printf("GAME: %s", message)
}

func checkChipConservation() bool {
	totalPlayerChips := 0
	for _, player := range gameState.Players {
		totalPlayerChips += player.Chips
	}
	totalChips := totalPlayerChips + gameState.Pot

	// Initialize the expected chips on first call
	if initialTotalChips == nil {
		initialTotalChips = &totalChips
	}

	if totalChips != *initialTotalChips {
		log.Printf("🚨 CHIP LEAK DETECTED! Expected: %d, Actual: %d", *initialTotalChips, totalChips)
		log.Printf("Player chips: %d, Pot: %d", totalPlayerChips, gameState.Pot)

		balances := make([]string, len(gameState.Players))
		for i, p := range gameState.Players {
			balances[i] = fmt.Sprintf("%s: %d", p.Name, p.Chips)
		}
		log.Printf("Player balances: %s", strings.Join(balances, ", "))
	}
	return totalChips == *initialTotalChips
}

func checkForEliminations() []string {
	var newlyEliminated []string

	for _, player := range gameState.Players {
		if player.Chips == 0 && !contains(gameState.EliminatedPlayers, player.Name) {
			gameState.EliminatedPlayers = append(gameState.EliminatedPlayers, player.Name)
			newlyEliminated = append(newlyEliminated, player.Name)
			addToLog(fmt.Sprintf("%s has been eliminated from the tournament!", player.Name))
		}
	}

	return newlyEliminated
}

func getActivePlayers() []Player {
	var active []Player
	for _, player := range gameState.Players {
		if !contains(gameState.EliminatedPlayers, player.Name) {
			active = append(active, player)
		}
	}
	return active
}

func checkForTournamentEnd() bool {
	activePlayers := getActivePlayers()

	if len(activePlayers) == 1 {
		gameState.GameEnded = true
		winner := activePlayers[0]
		addToLog(fmt.Sprintf("🏆 TOURNAMENT WINNER: %s wins with $%d! 🏆", winner.Name, winner.Chips))
		log.Printf("🏆 Tournament ended! Winner: %s with $%d", winner.Name, winner.Chips)
		return true
	}

	return false
}

func postBlinds() {
	activePlayers := getActivePlayers()

	if len(activePlayers) < 2 {
		addToLog("Not enough active players to post blinds")
		return
	}

	// Find dealer among active players
	dealerIndex := -1
	for i, p := range activePlayers {
		if p.Name == gameState.Players[gameState.DealerPosition].Name {
			dealerIndex = i
			break
		}
	}

	// If current dealer is eliminated, move to next active player
	if dealerIndex == -1 {
		for i := 1; i < len(gameState.Players); i++ {
			nextPos := (gameState.DealerPosition + i) % len(gameState.Players)
			nextPlayer := gameState.Players[nextPos]
			if !contains(gameState.EliminatedPlayers, nextPlayer.Name) {
				gameState.DealerPosition = nextPos
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
	for i := range gameState.Players {
		if gameState.Players[i].Name == smallBlindPlayer.Name {
			sbAmount := min(gameState.SmallBlind, gameState.Players[i].Chips)
			gameState.Players[i].Chips -= sbAmount
			gameState.Pot += sbAmount
			gameState.PlayerBets[gameState.Players[i].Name] = sbAmount
			addToLog(fmt.Sprintf("%s posts small blind $%d", gameState.Players[i].Name, sbAmount))
		}

		if gameState.Players[i].Name == bigBlindPlayer.Name {
			bbAmount := min(gameState.BigBlind, gameState.Players[i].Chips)
			gameState.Players[i].Chips -= bbAmount
			gameState.Pot += bbAmount
			gameState.PlayerBets[gameState.Players[i].Name] = bbAmount
			gameState.CurrentBet = bbAmount
			addToLog(fmt.Sprintf("%s posts big blind $%d", gameState.Players[i].Name, bbAmount))
		}
	}

	// Set current player to first active player after big blind
	firstToActIndex := (bigBlindIndex + 1) % len(activePlayers)
	firstToActPlayer := activePlayers[firstToActIndex]

	for i, p := range gameState.Players {
		if p.Name == firstToActPlayer.Name {
			gameState.CurrentPlayer = i
			break
		}
	}
}

func initializeDeck() []string {
	suits := []string{"♠", "♣", "♥", "♦"}
	values := []string{"2", "3", "4", "5", "6", "7", "8", "9", "10", "J", "Q", "K", "A"}
	var deck []string

	for _, suit := range suits {
		for _, value := range values {
			deck = append(deck, value+suit)
		}
	}

	return shuffle(deck)
}

func shuffle(array []string) []string {
	rand.Seed(time.Now().UnixNano())
	for i := len(array) - 1; i > 0; i-- {
		j := rand.Intn(i + 1)
		array[i], array[j] = array[j], array[i]
	}
	return array
}

func getPokerActionTool() PokerActionTool {
	tool := PokerActionTool{}
	tool.Type = "function"
	tool.Function.Name = "make_poker_action"
	tool.Function.Description = "Make a poker action decision (fold, call, check, or raise)"
	tool.Function.Parameters.Type = "object"
	tool.Function.Parameters.Properties.Action.Type = "string"
	tool.Function.Parameters.Properties.Action.Enum = []string{"fold", "call", "check", "raise"}
	tool.Function.Parameters.Properties.Action.Description = "The poker action to take"
	tool.Function.Parameters.Properties.RaiseAmount.Type = "number"
	tool.Function.Parameters.Properties.RaiseAmount.Description = "The total amount to raise to (only required if action is 'raise')"
	tool.Function.Parameters.Properties.RaiseAmount.Minimum = 0
	tool.Function.Parameters.Properties.Reasoning.Type = "string"
	tool.Function.Parameters.Properties.Reasoning.Description = "Brief explanation of the decision"
	tool.Function.Parameters.Required = []string{"action"}

	return tool
}

func getAIDecision(player Player) (string, error) {
	if OPENROUTER_API_KEY == "" {
		log.Printf("OPENROUTER_API_KEY is not set!")
		return "fold", fmt.Errorf("API key not configured")
	}

	playerCurrentBet := gameState.PlayerBets[player.Name]
	amountToCall := gameState.CurrentBet - playerCurrentBet
	minRaiseAmount := gameState.CurrentBet + gameState.MinRaise

	prompt := fmt.Sprintf(`You are playing Texas Hold'em Poker. Analyze your situation and make a decision.

Game State:
- Your cards: %s
- Community cards: %s
- Current pot: $%d
- Your chips: $%d
- Current bet: $%d
- Amount to call: $%d
- Minimum raise amount: $%d

Actions available:
- fold: Give up your hand and any money already bet
- call: Match the current bet by paying $%d
- check: Stay in the hand without betting (only when amount to call is \$0)
- raise: Increase the bet to a higher amount

Use the make_poker_action function to make your decision.`,
		strings.Join(player.Cards, ", "),
		strings.Join(gameState.CommunityCards, ", "),
		gameState.Pot,
		player.Chips,
		gameState.CurrentBet,
		amountToCall,
		minRaiseAmount,
		amountToCall)

	requestBody := OpenRouterRequest{
		Model: player.Model,
		Messages: []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		}{
			{Role: "user", Content: prompt},
		},
		Tools: []PokerActionTool{getPokerActionTool()},
		ToolChoice: map[string]interface{}{
			"type": "function",
			"function": map[string]string{
				"name": "make_poker_action",
			},
		},
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return "fold", err
	}

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("POST", OPENROUTER_BASE_URL+"/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return "fold", err
	}

	req.Header.Set("Authorization", "Bearer "+OPENROUTER_API_KEY)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("HTTP-Referer", "http://localhost:3000")
	req.Header.Set("X-Title", "AI Poker Arena")

	resp, err := client.Do(req)
	if err != nil {
		return "fold", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 429 {
		rateLimited = true
		go func() {
			time.Sleep(60 * time.Second)
			rateLimited = false
		}()
		return "fold", fmt.Errorf("rate limited")
	}

	var response OpenRouterResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "fold", err
	}

	if len(response.Choices) > 0 {
		message := response.Choices[0].Message

		// Check if the model used function calling
		if len(message.ToolCalls) > 0 {
			toolCall := message.ToolCalls[0]
			if toolCall.Function.Name == "make_poker_action" {
				var args ActionArgs
				if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err == nil {
					log.Printf("AI decision (%s): action=%s, raise_amount=%d, reasoning=%s",
						player.Model, args.Action, args.RaiseAmount, args.Reasoning)
					if args.Action == "raise" && args.RaiseAmount > 0 {
						return fmt.Sprintf("raise %d", args.RaiseAmount), nil
					}
					return args.Action, nil
				}
			}
		}

		// Fallback to text parsing
		if message.Content != "" {
			responseText := strings.TrimSpace(message.Content)
			log.Printf("AI decision (fallback): %s", responseText)

			if strings.Contains(responseText, "call") {
				return "call", nil
			}
			if strings.Contains(responseText, "raise") {
				// Simple regex alternative for Go
				parts := strings.Fields(responseText)
				for i, part := range parts {
					if part == "raise" && i+1 < len(parts) {
						if amount, err := strconv.Atoi(strings.Trim(parts[i+1], "$")); err == nil {
							return fmt.Sprintf("raise %d", amount), nil
						}
					}
				}
			}
			if strings.Contains(responseText, "fold") {
				return "fold", nil
			}
		}
	}

	return "fold", nil
}

func processDecision(decision string, playerIndex int) {
	player := &gameState.Players[playerIndex]
	playerCurrentBet := gameState.PlayerBets[player.Name]
	amountToCall := gameState.CurrentBet - playerCurrentBet

	if strings.HasPrefix(decision, "raise") {
		parts := strings.Fields(decision)
		raiseAmount := gameState.CurrentBet + gameState.MinRaise
		if len(parts) > 1 {
			if amount, err := strconv.Atoi(parts[1]); err == nil {
				raiseAmount = amount
			}
		}

		totalBet := max(raiseAmount, gameState.CurrentBet+gameState.MinRaise)
		actualRaiseAmount := totalBet - playerCurrentBet

		if actualRaiseAmount <= player.Chips {
			gameState.LastRaiseAmount = totalBet - gameState.CurrentBet
			gameState.CurrentBet = totalBet
			gameState.Pot += actualRaiseAmount
			player.Chips -= actualRaiseAmount
			gameState.PlayerBets[player.Name] = totalBet
			addToLog(fmt.Sprintf("%s raises to $%d (adding $%d)", player.Name, totalBet, actualRaiseAmount))
		} else {
			// If player can't afford raise, convert to call if possible
			if player.Chips >= amountToCall {
				processDecision("call", playerIndex)
			} else {
				processDecision("fold", playerIndex)
			}
		}
	} else if decision == "call" {
		if player.Chips >= amountToCall {
			gameState.Pot += amountToCall
			player.Chips -= amountToCall
			gameState.PlayerBets[player.Name] = gameState.CurrentBet
			addToLog(fmt.Sprintf("%s calls $%d", player.Name, amountToCall))
		} else {
			processDecision("fold", playerIndex)
		}
	} else if decision == "check" {
		if amountToCall == 0 {
			addToLog(fmt.Sprintf("%s checks", player.Name))
		} else {
			// Invalid check - convert to call or fold
			if player.Chips >= amountToCall {
				processDecision("call", playerIndex)
			} else {
				processDecision("fold", playerIndex)
			}
		}
	} else {
		addToLog(fmt.Sprintf("%s folds", player.Name))
		gameState.FoldedPlayers = append(gameState.FoldedPlayers, player.Name)
	}
}

func findFirstActivePlayerAfterDealer() int {
	for i := 1; i <= len(gameState.Players); i++ {
		nextPos := (gameState.DealerPosition + i) % len(gameState.Players)
		nextPlayer := gameState.Players[nextPos]
		if !contains(gameState.EliminatedPlayers, nextPlayer.Name) {
			return nextPos
		}
	}
	return gameState.DealerPosition // fallback
}

func advanceRound() {
	switch gameState.Round {
	case "preflop":
		gameState.Round = "flop"
		gameState.CurrentBet = 0
		gameState.PlayerBets = make(map[string]int)
		gameState.LastRaiseAmount = 0
		gameState.BettingComplete = false
		gameState.CurrentPlayer = findFirstActivePlayerAfterDealer()

		// Deal flop (pop from end like JavaScript)
		if len(gameState.Deck) >= 3 {
			gameState.CommunityCards = []string{
				gameState.Deck[len(gameState.Deck)-1],
				gameState.Deck[len(gameState.Deck)-2],
				gameState.Deck[len(gameState.Deck)-3],
			}
			gameState.Deck = gameState.Deck[:len(gameState.Deck)-3]
			addToLog(fmt.Sprintf("Flop dealt: %s", strings.Join(gameState.CommunityCards, ", ")))
		}

	case "flop":
		gameState.Round = "turn"
		gameState.CurrentBet = 0
		gameState.PlayerBets = make(map[string]int)
		gameState.LastRaiseAmount = 0
		gameState.BettingComplete = false
		gameState.CurrentPlayer = findFirstActivePlayerAfterDealer()

		// Deal turn
		if len(gameState.Deck) >= 1 {
			turnCard := gameState.Deck[len(gameState.Deck)-1]
			gameState.Deck = gameState.Deck[:len(gameState.Deck)-1]
			gameState.CommunityCards = append(gameState.CommunityCards, turnCard)
			addToLog(fmt.Sprintf("Turn dealt: %s", turnCard))
		}

	case "turn":
		gameState.Round = "river"
		gameState.CurrentBet = 0
		gameState.PlayerBets = make(map[string]int)
		gameState.LastRaiseAmount = 0
		gameState.BettingComplete = false
		gameState.CurrentPlayer = findFirstActivePlayerAfterDealer()

		// Deal river
		if len(gameState.Deck) >= 1 {
			riverCard := gameState.Deck[len(gameState.Deck)-1]
			gameState.Deck = gameState.Deck[:len(gameState.Deck)-1]
			gameState.CommunityCards = append(gameState.CommunityCards, riverCard)
			addToLog(fmt.Sprintf("River dealt: %s", riverCard))
		}

	case "river":
		endHand()
	}
}

func isBettingRoundComplete() bool {
	activePlayers := getActivePlayers()

	// Filter out folded players
	var activeUnfoldedPlayers []Player
	for _, player := range activePlayers {
		if !contains(gameState.FoldedPlayers, player.Name) {
			activeUnfoldedPlayers = append(activeUnfoldedPlayers, player)
		}
	}

	// All active players have acted and matched the current bet
	for _, player := range activeUnfoldedPlayers {
		playerBet := gameState.PlayerBets[player.Name]
		if playerBet < gameState.CurrentBet && player.Chips > 0 {
			return false // Player hasn't matched the bet and has chips
		}
	}
	return true
}

func endHand() {
	var activePlayers []Player
	for _, player := range gameState.Players {
		if !contains(gameState.FoldedPlayers, player.Name) {
			activePlayers = append(activePlayers, player)
		}
	}

	// If only one player remains, they win by default
	if len(activePlayers) == 1 {
		winner := &activePlayers[0]
		// Find the actual player in gameState to update chips
		for i := range gameState.Players {
			if gameState.Players[i].Name == winner.Name {
				gameState.Players[i].Chips += gameState.Pot
				addToLog(fmt.Sprintf("%s wins pot of $%d (all others folded)", winner.Name, gameState.Pot))
				break
			}
		}
	} else if len(activePlayers) == 0 {
		// All players folded - award pot to big blind position as a fallback
		bigBlindPos := (gameState.DealerPosition + 2) % len(gameState.Players)
		winner := &gameState.Players[bigBlindPos]
		winner.Chips += gameState.Pot
		addToLog(fmt.Sprintf("%s wins pot of $%d (all players folded, awarded to big blind)", winner.Name, gameState.Pot))
	} else {
		// Multiple players remain, compare hands
		hands := make([][]string, len(activePlayers))
		for i, player := range activePlayers {
			hands[i] = append(player.Cards, gameState.CommunityCards...)
		}

		winningHands := CompareHands(hands)
		if len(winningHands) > 0 {
			winningHand := winningHands[0]
			log.Printf("winningHand: %s", winningHand.GetHandName())

			// Find the winner by matching the exact hand
			winningCards := strings.Join(winningHand.CardStrings, "")
			for i := range gameState.Players {
				if !contains(gameState.FoldedPlayers, gameState.Players[i].Name) {
					playerCards := strings.Join(append(gameState.Players[i].Cards, gameState.CommunityCards...), "")
					if playerCards == winningCards {
						gameState.Players[i].Chips += gameState.Pot
						addToLog(fmt.Sprintf("%s wins pot of $%d with %s", gameState.Players[i].Name, gameState.Pot, winningHand.GetHandName()))
						break
					}
				}
			}
		}
	}

	// Log player balances at end of hand
	balances := make([]string, len(gameState.Players))
	for i, p := range gameState.Players {
		balances[i] = fmt.Sprintf("%s: $%d", p.Name, p.Chips)
	}
	addToLog(fmt.Sprintf("Hand #%d complete. Balances: %s", gameState.HandNumber, strings.Join(balances, ", ")))

	// Check for eliminations and tournament end
	checkForEliminations()
	checkForTournamentEnd()

	// Reset for next hand
	gameState.Pot = 0
	gameState.Round = "preflop"
	gameState.CommunityCards = []string{}
	gameState.FoldedPlayers = []string{}
	gameState.CurrentBet = 0
	gameState.PlayerBets = make(map[string]int)
	gameState.LastRaiseAmount = 0
	gameState.BettingComplete = false

	// Move dealer button to next active player
	remainingPlayers := getActivePlayers()
	if len(remainingPlayers) > 1 {
		for i := 1; i < len(gameState.Players); i++ {
			nextPos := (gameState.DealerPosition + i) % len(gameState.Players)
			nextPlayer := gameState.Players[nextPos]
			if !contains(gameState.EliminatedPlayers, nextPlayer.Name) {
				gameState.DealerPosition = nextPos
				break
			}
		}
	}
	gameState.HandNumber++

	// Clear player cards
	for i := range gameState.Players {
		gameState.Players[i].Cards = []string{}
	}
}

func advanceGame() {
	// Check if tournament has ended
	if gameState.GameEnded {
		return
	}

	// Deal new hand
	if gameState.Round == "preflop" &&
		len(gameState.CommunityCards) == 0 &&
		len(gameState.PlayerBets) == 0 {

		// Check if we have enough active players
		activePlayers := getActivePlayers()
		if len(activePlayers) < 2 {
			checkForTournamentEnd()
			return
		}

		// Initialize new hand
		gameState.Deck = initializeDeck()
		gameState.CurrentBet = 0
		gameState.PlayerBets = make(map[string]int)
		gameState.FoldedPlayers = []string{}
		gameState.BettingComplete = false

		// Deal cards only to active players
		for i := range gameState.Players {
			if !contains(gameState.EliminatedPlayers, gameState.Players[i].Name) {
				if len(gameState.Deck) >= 2 {
					gameState.Players[i].Cards = []string{
						gameState.Deck[len(gameState.Deck)-1],
						gameState.Deck[len(gameState.Deck)-2],
					}
					gameState.Deck = gameState.Deck[:len(gameState.Deck)-2]
				}
			}
		}

		// Post blinds
		postBlinds()
		checkChipConservation()
		addToLog(fmt.Sprintf("Hand #%d begins. Dealer: %s", gameState.HandNumber, gameState.Players[gameState.DealerPosition].Name))
	}

	currentPlayer := gameState.Players[gameState.CurrentPlayer]

	// Get current player's decision (skip if eliminated)
	if !contains(gameState.FoldedPlayers, currentPlayer.Name) &&
		!contains(gameState.EliminatedPlayers, currentPlayer.Name) {

		decision, err := getAIDecision(currentPlayer)
		if err != nil {
			log.Printf("Error getting AI decision: %v", err)
			decision = "fold"
		}
		processDecision(decision, gameState.CurrentPlayer)
		checkChipConservation()
	}

	// If all players have folded except one, end the hand
	activePlayers := getActivePlayers()
	activeUnfoldedPlayers := 0
	for _, player := range activePlayers {
		if !contains(gameState.FoldedPlayers, player.Name) {
			activeUnfoldedPlayers++
		}
	}
	if activeUnfoldedPlayers <= 1 {
		endHand()
		checkChipConservation()
		broadcastGameState()
		return
	}

	// Move to next player
	gameState.CurrentPlayer = (gameState.CurrentPlayer + 1) % len(gameState.Players)

	// Check if betting round is complete
	if isBettingRoundComplete() {
		advanceRound()
	}

	// Broadcast updated game state
	broadcastGameState()
}

func broadcastGameState() {
	for client := range clients {
		if err := client.WriteJSON(gameState); err != nil {
			log.Printf("Error broadcasting to client: %v", err)
			client.Close()
			delete(clients, client)
		}
	}
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}
	defer conn.Close()

	clients[conn] = true
	log.Println("Client connected")

	// Send initial game state
	if err := conn.WriteJSON(gameState); err != nil {
		log.Printf("Error sending initial game state: %v", err)
		delete(clients, conn)
		return
	}

	// Keep connection alive and handle disconnect
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			log.Printf("WebSocket read error: %v", err)
			delete(clients, conn)
			break
		}
	}
}

func gameLoop() {
	for !gameState.GameEnded {
		if !rateLimited {
			advanceGame()
		}
		time.Sleep(2 * time.Second)
	}

	log.Println("🏆 Tournament has ended! Game loop stopped.")
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

// HTTP handlers
func serveHome(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "public/index.html")
}

func main() {
	// Load environment variables from .env file
	err := godotenv.Load()
	if err != nil {
		log.Printf("Warning: Error loading .env file: %v", err)
	}

	// Load API key from environment
	OPENROUTER_API_KEY = os.Getenv("OPENROUTER_API_KEY")

	// Serve static files
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("public"))))

	// WebSocket endpoint
	http.HandleFunc("/ws", handleWebSocket)

	// Serve home page
	http.HandleFunc("/", serveHome)

	// Start game loop in a goroutine
	go gameLoop()

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	server := &http.Server{
		Addr: ":" + port,
	}

	// Channel to listen for interrupt signal
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Start server in a goroutine
	go func() {
		log.Printf("Server running on port %s", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Server failed to start:", err)
		}
	}()

	// Wait for interrupt signal
	<-stop
	log.Println("Shutting down server...")

	// Create a context with timeout for graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Attempt graceful shutdown
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	} else {
		log.Println("Server gracefully stopped")
	}
}
