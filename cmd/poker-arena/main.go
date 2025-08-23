package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/MikeLuu99/poker-arena/internal/game"
	"github.com/MikeLuu99/poker-arena/internal/server"
	"github.com/MikeLuu99/poker-arena/internal/tournament"
	"github.com/MikeLuu99/poker-arena/pkg/models"
	"github.com/joho/godotenv"
)

func main() {
	// Parse command line arguments
	config := parseFlags()
	
	// Show help and exit if requested
	if config.Help {
		flag.Usage()
		return
	}
	
	// Load environment variables from .env file
	err := godotenv.Load()
	if err != nil {
		log.Printf("Warning: Error loading .env file: %v", err)
	}
	
	// Set port from environment if not set by flag
	if config.Port == "" {
		config.Port = os.Getenv("PORT")
		if config.Port == "" {
			config.Port = "3000"
		}
	}
	
	// Initialize and run based on mode
	if config.Games > 1 {
		// Multiple games always use batch/tournament mode
		runBatchMode(config)
	} else {
		// Single game uses single game mode
		runSingleGameMode(config)
	}
}

func parseFlags() *models.Config {
	config := models.DefaultConfig()
	
	flag.IntVar(&config.Games, "games", config.Games, "Number of parallel games to run")
	flag.IntVar(&config.Games, "g", config.Games, "Number of parallel games to run (shorthand)")
	flag.StringVar(&config.OutputFile, "output", config.OutputFile, "CSV output file path") 
	flag.StringVar(&config.OutputFile, "o", config.OutputFile, "CSV output file path (shorthand)")
	flag.BoolVar(&config.NoServer, "no-server", config.NoServer, "Disable web server for batch mode")
	flag.BoolVar(&config.WithServers, "with-servers", config.WithServers, "Enable web servers for parallel games (ports 3000, 3001, 3002, ...)")
	flag.BoolVar(&config.Verbose, "verbose", config.Verbose, "Enable verbose logging")
	flag.BoolVar(&config.Verbose, "v", config.Verbose, "Enable verbose logging (shorthand)")
	flag.StringVar(&config.Port, "port", "", "Base web server port for parallel games (default: 3000 or PORT env var)")
	flag.BoolVar(&config.Help, "help", config.Help, "Show help information")
	flag.BoolVar(&config.Help, "h", config.Help, "Show help information (shorthand)")
	
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Poker Arena - AI Poker Tournament System\n\n")
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  %s                                    # Single game with web interface\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -g 10 -o results.csv --no-server  # 10 parallel games, save to CSV\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -g 3 --with-servers               # 3 parallel games with web UIs (ports 3000-3002)\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s --games 50 --verbose              # 50 games with progress logging\n", os.Args[0])
	}
	
	flag.Parse()
	return config
}

func runBatchMode(config *models.Config) {
	log.Printf("Starting batch tournament: %d games", config.Games)
	
	// Initialize tournament manager
	manager := tournament.NewGameManager(config)
	defer manager.Stop()
	
	// Channel to listen for interrupt signal
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	
	// Run tournament in goroutine
	tournamentChan := make(chan *models.TournamentResult, 1)
	go func() {
		result, err := manager.RunTournament()
		if err != nil {
			log.Printf("Tournament error: %v", err)
		}
		tournamentChan <- result
	}()
	
	// Wait for completion or interrupt
	select {
	case result := <-tournamentChan:
		printTournamentSummary(result)
	case <-stop:
		log.Println("Interrupt received. Stopping tournament...")
		manager.Stop()
		
		// Wait a bit for graceful shutdown
		select {
		case result := <-tournamentChan:
			log.Println("Tournament stopped. Partial results:")
			printTournamentSummary(result)
		case <-time.After(5 * time.Second):
			log.Println("Timeout waiting for tournament shutdown")
		}
	}
}

func runSingleGameMode(config *models.Config) {
	// Initialize single game
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
	
	httpServer := &http.Server{
		Addr:    ":" + config.Port,
		Handler: s.Router(),
	}
	
	// Channel to listen for interrupt signal
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	
	// Start server in a goroutine
	go func() {
		log.Printf("Server running on port %s", config.Port)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Server failed to start:", err)
		}
	}()
	
	// Wait for either game completion or interrupt signal
	select {
	case result := <-gameResultChan:
		if result != nil {
			printGameResult(result)
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

func printGameResult(result *models.GameResult) {
	log.Println("\n" + strings.Repeat("=", 60))
	log.Println("🏆 POKER GAME COMPLETED! 🏆")
	log.Println(strings.Repeat("=", 60))
	log.Printf("Winner: %s", result.Winner.Name)
	log.Printf("Final Chips: $%d", result.FinalChips)
	log.Printf("Total Hands: %d", result.TotalHands)
	log.Printf("Game Duration: %s", result.GameDuration)
	log.Printf("Eliminated Players: %v", result.Eliminated)
	log.Println(strings.Repeat("=", 60))
}

func printTournamentSummary(tournament *models.TournamentResult) {
	if tournament == nil {
		return
	}
	
	log.Println("\n" + strings.Repeat("=", 70))
	log.Println("🏆 TOURNAMENT COMPLETED! 🏆")
	log.Println(strings.Repeat("=", 70))
	log.Printf("Total Games: %d", tournament.CompletedGames)
	log.Printf("Tournament Duration: %s", tournament.TournamentDuration)
	log.Printf("Overall Winner: %s", tournament.OverallWinner)
	log.Println()
	log.Println("PLAYER STATISTICS:")
	log.Println(strings.Repeat("-", 70))
	
	for _, stats := range tournament.PlayerStats {
		log.Printf("%-25s | Wins: %2d | Win Rate: %5.1f%% | Avg Rank: %.2f",
			stats.Name, stats.Wins, stats.WinRate, stats.AvgRank)
	}
	
	log.Println(strings.Repeat("=", 70))
}