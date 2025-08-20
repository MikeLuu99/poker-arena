package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/MikeLuu99/poker-arena/pkg/models"
)

// OpenRouter API configuration
var (
	OPENROUTER_BASE_URL = "https://openrouter.ai/api/v1"
	rateLimited         = false
)

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

func GetAIDecision(player models.Player, gameState *models.GameState) (string, error) {
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		log.Printf("OPENROUTER_API_KEY is not set!")
		return "fold", fmt.Errorf("API key not configured")
	}

	if rateLimited {
		return "fold", fmt.Errorf("rate limited")
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
- check: Stay in the hand without betting (only when amount to call is $0)
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

	req.Header.Set("Authorization", "Bearer "+apiKey)
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