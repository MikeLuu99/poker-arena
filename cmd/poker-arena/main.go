package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/MikeLuu99/poker-arena/internal/game"
	"github.com/MikeLuu99/poker-arena/internal/server"
	"github.com/MikeLuu99/poker-arena/pkg/models"
	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables from .env file
	err := godotenv.Load()
	if err != nil {
		log.Printf("Warning: Error loading .env file: %v", err)
	}

	// Initialize game
	g := game.NewGame()

	// Initialize server
	s := server.NewServer(g)

	// Channel to receive game result
	gameResultChan := make(chan *models.GameResult, 1)

	// Start game loop in a goroutine
	go func() {
		result := g.Start()
		gameResultChan <- result
	}()

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	httpServer := &http.Server{
		Addr:    ":" + port,
		Handler: s.Router(),
	}

	// Channel to listen for interrupt signal
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Start server in a goroutine
	go func() {
		log.Printf("Server running on port %s", port)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Server failed to start:", err)
		}
	}()

	// Wait for either game completion or interrupt signal
	select {
	case result := <-gameResultChan:
		if result != nil {
			log.Println("\n" + strings.Repeat("=", 60))
			log.Println("🏆 POKER TOURNAMENT COMPLETED! 🏆")
			log.Println(strings.Repeat("=", 60))
			log.Printf("Winner: %s", result.Winner.Name)
			log.Printf("Final Chips: $%d", result.FinalChips)
			log.Printf("Total Hands: %d", result.TotalHands)
			log.Printf("Game Duration: %s", result.GameDuration)
			log.Printf("Eliminated Players: %v", result.Eliminated)
			log.Println(strings.Repeat("=", 60))
		}
		log.Println("Game completed. Shutting down server...")
	case <-stop:
		log.Println("Interrupt received. Shutting down server...")
		g.Stop()
	}

	// Create a context with timeout for graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Attempt graceful shutdown
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	} else {
		log.Println("Server gracefully stopped")
	}
}