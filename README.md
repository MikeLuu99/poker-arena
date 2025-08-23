# Poker Arena

A comprehensive AI poker tournament system where multiple AI models compete in Texas Hold'em poker. Run single games with a live web interface or execute large-scale parallel tournaments with detailed statistical analysis.

## Features

### Game Modes
- **Single Game Mode**: Interactive web interface with real-time game visualization
- **Parallel Tournament Mode**: Run multiple games simultaneously for statistical analysis
- **Parallel Web Mode**: Multiple games with individual web interfaces on sequential ports
- **Batch Processing**: Execute tournaments without GUI for data collection

### AI Competitors
- **Google Gemini 2.5 Flash**: Advanced reasoning and strategic play
- **OpenAI GPT-5 Nano**: Efficient decision-making model
- **OpenAI GPT OSS 120B**: Large-scale language model
- **Anthropic Claude 3.5 Haiku**: Fast and strategic AI player

*Note: You can use OpenRouter API to access different models for testing and comparison. Simply use your OpenRouter API key and specify model names in the format expected by OpenRouter.*

### Advanced Analytics
- **CSV Export**: Comprehensive game data and tournament statistics
- **Player Rankings**: Detailed performance metrics and win rates
- **Tournament Summaries**: Aggregate statistics across multiple games
- **Real-time Progress**: Live updates during parallel execution

### Additional Features
- **Clean Web Interface**: Minimalist design with real-time updates via HTMX
- **Multi-Port Web Serving**: Individual web interfaces for each parallel game
- **Detailed Logging**: Complete game action history and AI decision reasoning
- **Responsive Design**: Works seamlessly on desktop and mobile devices
- **Configurable Parameters**: Customizable game count, output files, and logging levels

## Tech Stack

- **Backend**: Go with Gorilla WebSocket and concurrent processing
- **Frontend**: HTML, CSS, JavaScript with HTMX for real-time updates
- **AI Integration**: REST API calls to various AI providers
- **Data Export**: Thread-safe CSV generation with comprehensive statistics
- **Architecture**: Clean architecture with separated concerns and parallel execution

## Project Structure

```
poker-arena/
├── cmd/
│   └── poker-arena/
│       └── main.go            # Application entry point with CLI support
├── internal/
│   ├── ai/
│   │   └── client.go          # AI model communication
│   ├── game/
│   │   ├── game.go            # Core game logic with ID support
│   │   └── actions.go         # Player actions (bet, fold, etc.)
│   ├── poker/
│   │   ├── deck.go            # Card deck management
│   │   └── hand.go            # Hand evaluation with safety checks
│   ├── server/
│   │   └── server.go          # HTTP server and API endpoints
│   └── tournament/
│       ├── manager.go         # Parallel game coordination
│       └── exporter.go        # CSV export functionality
├── pkg/
│   └── models/
│       ├── game.go            # Game data structures
│       ├── tournament.go      # Tournament aggregation models
│       └── config.go          # CLI configuration
├── index.html                 # Frontend web interface
├── go.mod                     # Go module dependencies
└── go.sum                     # Go dependency checksums
```

## Quick Start

### Prerequisites

- Go 1.24.2 or higher
- AI API keys from supported providers (OpenAI, Google, Anthropic, or OpenRouter for testing multiple models)

### Installation

1. Clone the repository:
   ```bash
   git clone https://github.com/MikeLuu99/poker-arena.git
   cd poker-arena
   ```

2. Install dependencies:
   ```bash
   go mod download
   ```

3. Create a `.env` file with your AI API keys:
   ```env
   # Add your AI API keys here
   OPENAI_API_KEY=your_openai_key
   GOOGLE_API_KEY=your_google_key
   ANTHROPIC_API_KEY=your_anthropic_key

   # Alternative: Use OpenRouter for testing different models
   # OPENAI_API_KEY=your_openrouter_key  # Use OpenRouter key for all OpenAI models
   # GOOGLE_API_KEY=your_openrouter_key  # Use OpenRouter key for Google models
   # ANTHROPIC_API_KEY=your_openrouter_key  # Use OpenRouter key for Anthropic models

   PORT=3000
   ```

## Usage Examples

### Single Game with Web Interface (Default)
```bash
# Run single game with live web interface
go run cmd/poker-arena/main.go

# Open browser to http://localhost:3000
```

### Parallel Tournament Mode
```bash
# Run 10 parallel games and export to CSV (batch mode)
go run cmd/poker-arena/main.go --games 10 --output tournament_results.csv --no-server

# Run 3 parallel games with web interfaces on ports 3000-3002
go run cmd/poker-arena/main.go --games 3 --with-servers --verbose

# Run 50 games with verbose progress logging (batch mode)
go run cmd/poker-arena/main.go -g 50 -v --no-server

# Run batch tournament with custom output file
go run cmd/poker-arena/main.go -g 25 -o ai_poker_analysis.csv --no-server --verbose
```

### Command Line Options

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--games` | `-g` | Number of parallel games to run | 1 |
| `--output` | `-o` | CSV output file path | `poker_results.csv` |
| `--no-server` | | Disable web server (batch mode) | false |
| `--with-servers` | | Enable web servers for parallel games | false |
| `--verbose` | `-v` | Enable detailed logging | false |
| `--port` | | Base web server port for parallel games | 3000 |
| `--help` | `-h` | Show help information | |

### Environment Configuration

| Variable | Description | Required |
|----------|-------------|----------|
| `OPENAI_API_KEY` | OpenAI API key for GPT models (or OpenRouter key) | Yes |
| `GOOGLE_API_KEY` | Google API key for Gemini (or OpenRouter key) | Yes |
| `ANTHROPIC_API_KEY` | Anthropic API key for Claude (or OpenRouter key) | Yes |
| `PORT` | Web server port (overridden by --port flag) | No |

### Testing with OpenRouter

OpenRouter provides access to multiple AI models through a single API key, making it ideal for testing and comparing different models:

1. **Sign up for OpenRouter**: Visit [openrouter.ai](https://openrouter.ai) and get your API key
2. **Use OpenRouter key**: Set all three API keys to your OpenRouter key in the `.env` file
3. **Model compatibility**: The current model names work with OpenRouter's API format
4. **Cost-effective testing**: OpenRouter often provides competitive pricing for model testing

## How It Works

### Single Game Mode
1. **Game Initialization**: Four AI players start with equal chip stacks (20 chips each)
2. **Real-time Web Interface**: Live game visualization with player positions around virtual table
3. **AI Decision Making**: Each model receives game state and makes strategic decisions
4. **Live Updates**: Web interface refreshes every 2 seconds via HTMX
5. **Game Completion**: Continues until one player eliminates all others

### Parallel Tournament Mode
1. **Tournament Setup**: Configurable number of games run simultaneously
2. **Parallel Execution**: Worker pool manages concurrent games with resource limits
3. **Web Interface Options**: Choose between batch mode or individual web servers per game
4. **Data Collection**: Each game result captured with detailed statistics
5. **Progress Monitoring**: Real-time progress updates and completion tracking
6. **Statistical Analysis**: Aggregate data across all games for comprehensive insights

### Parallel Web Mode
1. **Multi-Port Serving**: Each game gets its own web server on sequential ports (3000, 3001, 3002, etc.)
2. **Individual Game Monitoring**: Watch specific games in real-time while others run in parallel
3. **Concurrent Visualization**: Multiple browser tabs for simultaneous game observation
4. **Resource Management**: Controlled concurrency with graceful server shutdown

### AI Decision Process
- **Game State Analysis**: Each AI receives complete game information
- **Strategic Reasoning**: Models consider hand strength, pot odds, player behavior
- **Action Selection**: Choose from fold, call, check, or raise with reasoning
- **Adaptive Play**: AI strategies evolve based on opponent patterns

## Game Rules & Mechanics

### Texas Hold'em Poker Rules
- Standard poker hand rankings and betting structure
- Four betting rounds: preflop, flop, turn, river
- Community cards shared by all players
- Winner determined by best 5-card poker hand

### Tournament Configuration
- **Blinds**: Small blind (5 chips), Big blind (10 chips)
- **Elimination**: Players eliminated when chips reach zero
- **Victory Condition**: Last player with chips wins the game

### Performance Metrics
- **Individual Games**: Winner, duration, total hands, final chip counts
- **Tournament Aggregation**: Win rates, average rankings, statistical significance
- **Player Analytics**: Head-to-head performance, strategy effectiveness

## CSV Export Format

The tournament system generates comprehensive CSV files with the following structure:

### Game Data
```csv
GameID,Winner,WinnerChips,TotalHands,GameDuration,StartTime,EndTime,Player1_Name,Player1_FinalChips,Player1_Rank,Player1_Position,...
```

### Tournament Summary
- Total games completed
- Overall tournament duration
- Overall winner (most wins)

### Player Statistics
- Win counts and win rates
- Average final rankings
- Average chip counts
- Head-to-head performance metrics

### Common Issues

**API Rate Limiting**
```bash
# If you encounter rate limiting, reduce concurrency:
go run cmd/poker-arena/main.go -g 5 -v  # Reduce from default max
```

**Memory Usage with Large Tournaments**
```bash
# Monitor system resources during large tournaments:
go run cmd/poker-arena/main.go -g 100 -v --no-server
```

**AI Response Timeouts**
- Check your internet connection and API key validity
- Some AI models may have slower response times during peak hours
- The system will automatically retry failed requests

### Performance Tuning

- **Optimal Concurrency**: 5-10 parallel games work well for most systems
- **CSV File Size**: Large tournaments (1000+ games) generate substantial data files
- **API Costs**: Monitor usage across AI providers as costs can accumulate quickly
