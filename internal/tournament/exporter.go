package tournament

import (
	"encoding/csv"
	"fmt"
	"os"
	"sync"

	"github.com/MikeLuu99/poker-arena/pkg/models"
)

// CSVExporter handles writing game results to CSV format
type CSVExporter struct {
	file   *os.File
	writer *csv.Writer
	mu     sync.Mutex
	header []string
}

// NewCSVExporter creates a new CSV exporter
func NewCSVExporter(filename string) (*CSVExporter, error) {
	file, err := os.Create(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to create CSV file: %w", err)
	}
	
	writer := csv.NewWriter(file)
	
	// Define CSV header
	header := []string{
		"GameID",
		"Winner",
		"WinnerChips",
		"TotalHands", 
		"GameDuration",
		"StartTime",
		"EndTime",
	}
	
	// Add columns for each player (assuming 4 players)
	playerColumns := []string{"Name", "FinalChips", "Rank", "Position"}
	for i := 1; i <= 4; i++ {
		for _, col := range playerColumns {
			header = append(header, fmt.Sprintf("Player%d_%s", i, col))
		}
	}
	
	exporter := &CSVExporter{
		file:   file,
		writer: writer,
		header: header,
	}
	
	// Write header
	if err := exporter.writer.Write(header); err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to write CSV header: %w", err)
	}
	exporter.writer.Flush()
	
	return exporter, nil
}

// WriteResult writes a single game result to the CSV file
func (e *CSVExporter) WriteResult(result *models.GameResult) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	
	// Basic game information
	record := []string{
		fmt.Sprintf("%d", result.GameID),
		result.Winner.Name,
		fmt.Sprintf("%d", result.FinalChips),
		fmt.Sprintf("%d", result.TotalHands),
		result.GameDuration,
		result.StartTime.Format("2006-01-02 15:04:05"),
		result.EndTime.Format("2006-01-02 15:04:05"),
	}
	
	// Add player ranking data (pad to 4 players)
	rankings := result.PlayerRankings
	for i := 0; i < 4; i++ {
		if i < len(rankings) {
			ranking := rankings[i]
			record = append(record,
				ranking.Player.Name,
				fmt.Sprintf("%d", ranking.Player.Chips),
				fmt.Sprintf("%d", ranking.Rank),
				ranking.Position,
			)
		} else {
			// Empty data for missing players
			record = append(record, "", "0", "0", "")
		}
	}
	
	if err := e.writer.Write(record); err != nil {
		return fmt.Errorf("failed to write CSV record: %w", err)
	}
	
	e.writer.Flush()
	return e.writer.Error()
}

// WriteSummary writes tournament summary statistics
func (e *CSVExporter) WriteSummary(tournament *models.TournamentResult) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	
	// Write empty line separator
	e.writer.Write([]string{})
	
	// Write summary header
	summaryHeader := []string{
		"TOURNAMENT SUMMARY",
		"TotalGames",
		"CompletedGames", 
		"TournamentDuration",
		"OverallWinner",
	}
	e.writer.Write(summaryHeader)
	
	// Write summary data
	summaryData := []string{
		"",
		fmt.Sprintf("%d", tournament.TotalGames),
		fmt.Sprintf("%d", tournament.CompletedGames),
		tournament.TournamentDuration,
		tournament.OverallWinner,
	}
	e.writer.Write(summaryData)
	
	// Write player statistics header
	e.writer.Write([]string{})
	playerStatsHeader := []string{
		"PLAYER STATISTICS",
		"PlayerName",
		"TotalGames",
		"Wins",
		"SecondPlace", 
		"ThirdPlace",
		"FourthPlace",
		"WinRate%",
		"AvgRank",
		"AvgChips",
	}
	e.writer.Write(playerStatsHeader)
	
	// Write each player's statistics
	for _, stats := range tournament.PlayerStats {
		playerRecord := []string{
			"",
			stats.Name,
			fmt.Sprintf("%d", stats.TotalGames),
			fmt.Sprintf("%d", stats.Wins),
			fmt.Sprintf("%d", stats.SecondPlace),
			fmt.Sprintf("%d", stats.ThirdPlace),
			fmt.Sprintf("%d", stats.FourthPlace),
			fmt.Sprintf("%.2f", stats.WinRate),
			fmt.Sprintf("%.2f", stats.AvgRank),
			fmt.Sprintf("%.2f", stats.AvgChips),
		}
		e.writer.Write(playerRecord)
	}
	
	e.writer.Flush()
	return e.writer.Error()
}

// Close closes the CSV file and flushes any remaining data
func (e *CSVExporter) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	
	if e.writer != nil {
		e.writer.Flush()
	}
	
	if e.file != nil {
		return e.file.Close()
	}
	
	return nil
}