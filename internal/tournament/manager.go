package tournament

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/MikeLuu99/poker-arena/internal/game"
	"github.com/MikeLuu99/poker-arena/internal/server"
	"github.com/MikeLuu99/poker-arena/pkg/models"
)

// GameManager coordinates multiple parallel poker games
type GameManager struct {
	config     *models.Config
	tournament *models.TournamentResult
	exporter   *CSVExporter
	servers    []*http.Server
	mu         sync.RWMutex
	ctx        context.Context
	cancel     context.CancelFunc
}

// NewGameManager creates a new game manager
func NewGameManager(config *models.Config) *GameManager {
	ctx, cancel := context.WithCancel(context.Background())
	
	tournament := models.NewTournamentResult(config.Games)
	
	var exporter *CSVExporter
	if config.OutputFile != "" {
		var err error
		exporter, err = NewCSVExporter(config.OutputFile)
		if err != nil {
			log.Printf("Warning: Failed to create CSV exporter: %v", err)
		}
	}
	
	return &GameManager{
		config:     config,
		tournament: tournament,
		exporter:   exporter,
		servers:    make([]*http.Server, 0),
		ctx:        ctx,
		cancel:     cancel,
	}
}

// RunTournament runs the configured number of games
func (gm *GameManager) RunTournament() (*models.TournamentResult, error) {
	if gm.config.Games == 1 {
		return gm.runSingleGame()
	}
	return gm.runParallelGames()
}

// runSingleGame runs a single game (existing behavior)
func (gm *GameManager) runSingleGame() (*models.TournamentResult, error) {
	if gm.config.Verbose {
		log.Println("Starting single game...")
	}
	
	g := game.NewGameWithID(1)
	result := g.Start()
	
	if result != nil {
		// Populate additional fields
		result.StartTime = g.GetStartTime()
		result.EndTime = time.Now()
		result.PlayerRankings = gm.calculatePlayerRankings(result)
		
		gm.tournament.AddGameResult(result)
		
		if gm.exporter != nil {
			if err := gm.exporter.WriteResult(result); err != nil {
				log.Printf("Error writing to CSV: %v", err)
			}
		}
	}
	
	return gm.tournament, nil
}

// runParallelGames runs multiple games in parallel
func (gm *GameManager) runParallelGames() (*models.TournamentResult, error) {
	if gm.config.Verbose {
		log.Printf("Starting tournament with %d parallel games...", gm.config.Games)
	}
	
	// Channel to collect results
	resultsChan := make(chan *models.GameResult, gm.config.Games)
	
	// Create all games first
	games := make([]*game.Game, gm.config.Games)
	for i := 0; i < gm.config.Games; i++ {
		games[i] = game.NewGameWithID(i + 1)
	}
	
	// Start web servers if requested
	if gm.config.WithServers {
		gm.startWebServersForGames(games)
	}
	
	// Worker pool for parallel games
	maxWorkers := min(gm.config.Games, 10) // Limit concurrent games to avoid resource exhaustion
	workerSem := make(chan struct{}, maxWorkers)
	
	var wg sync.WaitGroup
	
	// Start progress reporting if verbose
	if gm.config.Verbose {
		go gm.reportProgress()
	}
	
	// Launch all games
	for i, g := range games {
		wg.Add(1)
		go func(g *game.Game, gameID int) {
			defer wg.Done()
			
			// Acquire worker slot
			workerSem <- struct{}{}
			defer func() { <-workerSem }()
			
			// Check if context is cancelled
			select {
			case <-gm.ctx.Done():
				return
			default:
			}
			
			if gm.config.Verbose {
				log.Printf("Starting game %d", gameID)
			}
			
			// Run the game
			result := g.Start()
			
			if result != nil && gm.ctx.Err() == nil {
				// Populate additional fields
				result.StartTime = g.GetStartTime()
				result.EndTime = time.Now()
				result.PlayerRankings = gm.calculatePlayerRankings(result)
				
				resultsChan <- result
				
				if gm.config.Verbose {
					log.Printf("Game %d completed: Winner %s (%d hands, %s)", 
						gameID, result.Winner.Name, result.TotalHands, result.GameDuration)
				}
			}
		}(g, i+1)
	}
	
	// Close results channel when all workers are done
	go func() {
		wg.Wait()
		close(resultsChan)
	}()
	
	// Collect results
	for result := range resultsChan {
		select {
		case <-gm.ctx.Done():
			return gm.tournament, gm.ctx.Err()
		default:
		}
		
		gm.mu.Lock()
		gm.tournament.AddGameResult(result)
		gm.mu.Unlock()
		
		// Write to CSV
		if gm.exporter != nil {
			if err := gm.exporter.WriteResult(result); err != nil {
				log.Printf("Error writing to CSV: %v", err)
			}
		}
	}
	
	if gm.config.Verbose && gm.tournament.IsComplete() {
		log.Printf("Tournament completed! Overall winner: %s", gm.tournament.OverallWinner)
	}
	
	return gm.tournament, nil
}

// calculatePlayerRankings determines final rankings based on chip count
func (gm *GameManager) calculatePlayerRankings(result *models.GameResult) []models.PlayerRanking {
	// Sort players by chip count (descending)
	players := make([]models.Player, len(result.AllPlayers))
	copy(players, result.AllPlayers)
	
	// Simple bubble sort by chips
	for i := 0; i < len(players)-1; i++ {
		for j := 0; j < len(players)-1-i; j++ {
			if players[j].Chips < players[j+1].Chips {
				players[j], players[j+1] = players[j+1], players[j]
			}
		}
	}
	
	rankings := make([]models.PlayerRanking, len(players))
	positions := []string{"Winner", "Runner-up", "3rd Place", "4th Place"}
	
	for i, player := range players {
		position := "4th Place"
		if i < len(positions) {
			position = positions[i]
		}
		
		rankings[i] = models.PlayerRanking{
			Player:   player,
			Rank:     i + 1,
			Position: position,
		}
	}
	
	return rankings
}

// reportProgress reports tournament progress periodically
func (gm *GameManager) reportProgress() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-gm.ctx.Done():
			return
		case <-ticker.C:
			gm.mu.RLock()
			progress := gm.tournament.GetProgress()
			completed := gm.tournament.CompletedGames
			total := gm.tournament.TotalGames
			gm.mu.RUnlock()
			
			if completed < total {
				log.Printf("Tournament progress: %d/%d games completed (%.1f%%)", 
					completed, total, progress)
			}
		}
	}
}

// startWebServersForGames starts HTTP servers for the provided games
func (gm *GameManager) startWebServersForGames(games []*game.Game) {
	basePort, err := strconv.Atoi(gm.config.Port)
	if err != nil {
		basePort = 3000
	}
	
	if gm.config.Verbose {
		log.Printf("Starting %d web servers on ports %d-%d", len(games), basePort, basePort+len(games)-1)
	}
	
	for i, g := range games {
		port := basePort + i
		gameID := g.ID
		
		// Create server for this game
		s := server.NewServer(g)
		httpServer := &http.Server{
			Addr:    fmt.Sprintf(":%d", port),
			Handler: s.Router(),
		}
		
		gm.mu.Lock()
		gm.servers = append(gm.servers, httpServer)
		gm.mu.Unlock()
		
		// Start server in goroutine
		go func(srv *http.Server, gameID int, port int) {
			if gm.config.Verbose {
				log.Printf("Web server for game %d running on port %d", gameID, port)
			}
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Printf("Game %d server error: %v", gameID, err)
			}
		}(httpServer, gameID, port)
	}
	
	if gm.config.Verbose {
		log.Printf("All web servers started. Access games at:")
		for i := 0; i < len(games); i++ {
			log.Printf("  Game %d: http://localhost:%d", i+1, basePort+i)
		}
	}
}

// stopWebServers gracefully shuts down all web servers
func (gm *GameManager) stopWebServers() {
	if len(gm.servers) == 0 {
		return
	}
	
	if gm.config.Verbose {
		log.Printf("Stopping %d web servers...", len(gm.servers))
	}
	
	for i, srv := range gm.servers {
		go func(server *http.Server, index int) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			
			if err := server.Shutdown(ctx); err != nil {
				if gm.config.Verbose {
					log.Printf("Server %d forced shutdown: %v", index+1, err)
				}
			} else if gm.config.Verbose {
				log.Printf("Server %d stopped gracefully", index+1)
			}
		}(srv, i)
	}
}

// Stop gracefully stops the tournament
func (gm *GameManager) Stop() {
	log.Println("Stopping tournament...")
	gm.cancel()
	
	// Stop web servers
	gm.stopWebServers()
	
	if gm.exporter != nil {
		gm.exporter.Close()
	}
}

// GetTournamentResult returns the current tournament result
func (gm *GameManager) GetTournamentResult() *models.TournamentResult {
	gm.mu.RLock()
	defer gm.mu.RUnlock()
	return gm.tournament
}

// Helper function for min
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}