package models

import "time"

// PlayerStats holds aggregated statistics for a player across multiple games
type PlayerStats struct {
	Name         string  `json:"name"`
	TotalGames   int     `json:"totalGames"`
	Wins         int     `json:"wins"`
	SecondPlace  int     `json:"secondPlace"`
	ThirdPlace   int     `json:"thirdPlace"`
	FourthPlace  int     `json:"fourthPlace"`
	WinRate      float64 `json:"winRate"`
	AvgRank      float64 `json:"avgRank"`
	TotalChips   int     `json:"totalChips"`   // Total chips won across all games
	AvgChips     float64 `json:"avgChips"`     // Average final chips per game
}

// TournamentResult holds aggregated results from multiple games
type TournamentResult struct {
	TotalGames       int                    `json:"totalGames"`
	CompletedGames   int                    `json:"completedGames"`
	StartTime        time.Time              `json:"startTime"`
	EndTime          time.Time              `json:"endTime"`
	TournamentDuration string               `json:"tournamentDuration"`
	GameResults      []*GameResult          `json:"gameResults"`
	PlayerStats      map[string]*PlayerStats `json:"playerStats"`
	OverallWinner    string                 `json:"overallWinner"` // Player with most wins
}

// NewTournamentResult creates a new tournament result tracker
func NewTournamentResult(totalGames int) *TournamentResult {
	return &TournamentResult{
		TotalGames:     totalGames,
		CompletedGames: 0,
		StartTime:      time.Now(),
		GameResults:    make([]*GameResult, 0, totalGames),
		PlayerStats:    make(map[string]*PlayerStats),
	}
}

// AddGameResult adds a completed game result to the tournament
func (tr *TournamentResult) AddGameResult(result *GameResult) {
	tr.GameResults = append(tr.GameResults, result)
	tr.CompletedGames++
	
	// Update player statistics
	for _, ranking := range result.PlayerRankings {
		playerName := ranking.Player.Name
		
		// Initialize player stats if not exists
		if _, exists := tr.PlayerStats[playerName]; !exists {
			tr.PlayerStats[playerName] = &PlayerStats{
				Name:        playerName,
				TotalGames:  0,
				Wins:        0,
				SecondPlace: 0,
				ThirdPlace:  0,
				FourthPlace: 0,
			}
		}
		
		stats := tr.PlayerStats[playerName]
		stats.TotalGames++
		stats.TotalChips += ranking.Player.Chips
		
		// Update placement counts
		switch ranking.Rank {
		case 1:
			stats.Wins++
		case 2:
			stats.SecondPlace++
		case 3:
			stats.ThirdPlace++
		case 4:
			stats.FourthPlace++
		}
		
		// Recalculate averages
		stats.WinRate = float64(stats.Wins) / float64(stats.TotalGames) * 100
		stats.AvgChips = float64(stats.TotalChips) / float64(stats.TotalGames)
		stats.AvgRank = (float64(stats.Wins)*1 + float64(stats.SecondPlace)*2 + 
						 float64(stats.ThirdPlace)*3 + float64(stats.FourthPlace)*4) / 
						 float64(stats.TotalGames)
	}
	
	// Update overall winner if tournament is complete
	if tr.IsComplete() {
		tr.EndTime = time.Now()
		tr.TournamentDuration = tr.EndTime.Sub(tr.StartTime).String()
		tr.updateOverallWinner()
	}
}

// IsComplete returns true if all games have been completed
func (tr *TournamentResult) IsComplete() bool {
	return tr.CompletedGames >= tr.TotalGames
}

// updateOverallWinner finds the player with the most wins
func (tr *TournamentResult) updateOverallWinner() {
	maxWins := 0
	for _, stats := range tr.PlayerStats {
		if stats.Wins > maxWins {
			maxWins = stats.Wins
			tr.OverallWinner = stats.Name
		}
	}
}

// GetProgress returns the completion percentage
func (tr *TournamentResult) GetProgress() float64 {
	if tr.TotalGames == 0 {
		return 0
	}
	return float64(tr.CompletedGames) / float64(tr.TotalGames) * 100
}