package models

// Config holds the application configuration
type Config struct {
	// Number of parallel games to run
	Games int
	
	// CSV output file path
	OutputFile string
	
	// Whether to disable the web server (batch mode)
	NoServer bool
	
	// Enable web servers for parallel games
	WithServers bool
	
	// Enable verbose logging
	Verbose bool
	
	// Web server port (base port for parallel games)
	Port string
	
	// Show help
	Help bool
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		Games:       1,
		OutputFile:  "poker_results.csv",
		NoServer:    false,
		WithServers: false,
		Verbose:     false,
		Port:        "3000",
		Help:        false,
	}
}

// IsBatchMode returns true if running in batch mode (no web server)
func (c *Config) IsBatchMode() bool {
	return c.NoServer || (c.Games > 1 && !c.WithServers)
}

// IsParallel returns true if running multiple games in parallel
func (c *Config) IsParallel() bool {
	return c.Games > 1
}