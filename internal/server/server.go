package server

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/MikeLuu99/poker-arena/internal/game"
	"github.com/gorilla/websocket"
)

type Server struct {
	game     *game.Game
	clients  map[*websocket.Conn]bool
	upgrader websocket.Upgrader
}

func NewServer(g *game.Game) *Server {
	return &Server{
		game:    g,
		clients: make(map[*websocket.Conn]bool),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins in development
			},
		},
	}
}

func (s *Server) Router() http.Handler {
	mux := http.NewServeMux()

	// Serve static files
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("public"))))

	// WebSocket endpoint
	mux.HandleFunc("/ws", s.handleWebSocket)

	// HTMX endpoints
	mux.HandleFunc("/game-state", s.handleGameState)

	// Serve home page
	mux.HandleFunc("/", s.serveHome)

	return mux
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}
	defer conn.Close()

	s.clients[conn] = true
	log.Println("Client connected")

	// Send initial game state
	if err := conn.WriteJSON(s.game.State); err != nil {
		log.Printf("Error sending initial game state: %v", err)
		delete(s.clients, conn)
		return
	}

	// Keep connection alive and handle disconnect
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			log.Printf("WebSocket read error: %v", err)
			delete(s.clients, conn)
			break
		}
	}
}

func (s *Server) BroadcastGameState() {
	for client := range s.clients {
		if err := client.WriteJSON(s.game.State); err != nil {
			log.Printf("Error broadcasting to client: %v", err)
			client.Close()
			delete(s.clients, client)
		}
	}
}

func (s *Server) serveHome(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "index.html")
}


func (s *Server) handleGameState(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(s.game.State); err != nil {
		http.Error(w, "Failed to encode game state", http.StatusInternalServerError)
		return
	}
}