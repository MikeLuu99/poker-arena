# Poker Arena

A real-time poker game where AI models compete against each other in Texas Hold'em poker. Watch as different AI models from OpenAI, Google, and Anthropic battle it out at the virtual poker table.

## Features

- **Real-time Poker Game**: Live Texas Hold'em poker simulation
- **AI vs AI**: Multiple AI models compete against each other:
  - Google Gemini 2.5 Flash
  - OpenAI GPT-5 Nano
  - OpenAI GPT OSS 120B
  - Anthropic Claude 3.5 Haiku
- **Live Web Interface**: Clean, minimalist web UI with real-time updates
- **Game Logging**: Detailed game log showing all actions and decisions
- **Responsive Design**: Works on desktop and mobile devices

## Tech Stack

- **Backend**: Go with Gorilla WebSocket
- **Frontend**: HTML, CSS, JavaScript with HTMX for real-time updates
- **AI Integration**: REST API calls to various AI providers
- **Architecture**: Clean architecture with separated concerns

## Project Structure

```
poker-arena/
├── cmd/
│   └── poker-arena/
│       └── main.go          # Application entry point
├── internal/
│   ├── ai/
│   │   └── client.go        # AI model communication
│   ├── game/
│   │   ├── game.go          # Core game logic
│   │   └── actions.go       # Player actions (bet, fold, etc.)
│   ├── poker/
│   │   ├── deck.go          # Card deck management
│   │   └── hand.go          # Hand evaluation
│   └── server/
│       └── server.go        # HTTP server and API endpoints
├── pkg/
│   └── models/
│       └── game.go          # Data models and structures
├── index.html               # Frontend web interface
├── go.mod                   # Go module dependencies
└── go.sum                   # Go dependency checksums
```

## Getting Started

### Prerequisites

- Go 1.24.2 or higher
- Environment variables for AI API keys (see Configuration)

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
   PORT=3000
   ```

4. Run the application:
   ```bash
   go run cmd/poker-arena/main.go
   ```

5. Open your browser and navigate to `http://localhost:3000`

### Configuration

The application uses environment variables for configuration:

- `PORT`: Server port (default: 3000)
- AI API keys for the respective providers
- Additional configuration can be added to the `.env` file

## How It Works

1. **Game Initialization**: Four AI players are created with initial chip stacks
2. **Game Loop**: The game runs continuously, dealing cards and managing betting rounds
3. **AI Decision Making**: Each AI model receives game state and makes decisions (fold, call, raise)
4. **Real-time Updates**: The web interface updates every 2 seconds using HTMX
5. **Game Logging**: All actions are logged and displayed in the game log panel

## Game Rules

- Standard Texas Hold'em poker rules
- Each player starts with 20 chips
- Blinds and betting structure follow standard poker conventions
- Game continues until only one player remains or manually stopped

## Development

### Building

```bash
go build -o poker-arena cmd/poker-arena/main.go
```

### Running Tests

```bash
go test ./...
```

### Code Structure

The project follows Go best practices with clean architecture:

- `cmd/`: Application entry points
- `internal/`: Private application code
- `pkg/`: Public library code that can be imported
- Separation of concerns between game logic, AI communication, and web serving
