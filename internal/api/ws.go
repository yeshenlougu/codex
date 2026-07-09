package api

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// wsClient is one connected WebSocket client.
type wsClient struct {
	conn      *websocket.Conn
	sessionID string
	send      chan []byte
	hub       *wsHub
}

// wsHub manages all WebSocket connections.
type wsHub struct {
	clients    map[*wsClient]bool
	broadcast  chan wsMessage
	register   chan *wsClient
	unregister chan *wsClient
	mu         sync.RWMutex
}

type wsMessage struct {
	Type      string `json:"type"` // "chunk", "tool_call", "done", "error"
	SessionID string `json:"session_id"`
	Content   string `json:"content,omitempty"`
	ToolName  string `json:"tool_name,omitempty"`
	ToolArgs  string `json:"tool_args,omitempty"`
	ToolOut   string `json:"tool_output,omitempty"`
	Error     string `json:"error,omitempty"`
}

func newWSHub() *wsHub {
	return &wsHub{
		clients:    make(map[*wsClient]bool),
		broadcast:  make(chan wsMessage, 256),
		register:   make(chan *wsClient),
		unregister: make(chan *wsClient),
	}
}

func (h *wsHub) run() {
	for {
		select {
		case c := <-h.register:
			h.mu.Lock()
			h.clients[c] = true
			h.mu.Unlock()

		case c := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[c]; ok {
				delete(h.clients, c)
				close(c.send)
			}
			h.mu.Unlock()

		case msg := <-h.broadcast:
			data, err := json.Marshal(msg)
			if err != nil {
				continue
			}
			h.mu.RLock()
			for c := range h.clients {
				if c.sessionID == msg.SessionID || msg.SessionID == "" {
					select {
					case c.send <- data:
					default:
						go h.unregisterClient(c)
					}
				}
			}
			h.mu.RUnlock()
		}
	}
}

func (h *wsHub) unregisterClient(c *wsClient) {
	h.unregister <- c
}

func (h *wsHub) broadcastMsg(msg wsMessage) {
	h.broadcast <- msg
}

// Sessions returns list of connected session IDs.
func (h *wsHub) Sessions() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	seen := make(map[string]bool)
	for c := range h.clients {
		if c.sessionID != "" {
			seen[c.sessionID] = true
		}
	}
	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	return ids
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[ws] upgrade error: %v", err)
		return
	}

	sessionID := r.URL.Query().Get("session_id")

	client := &wsClient{
		conn:      conn,
		sessionID: sessionID,
		send:      make(chan []byte, 64),
		hub:       s.wsHub,
	}
	s.wsHub.register <- client

	// Write pump
	go func() {
		defer func() {
			s.wsHub.unregister <- client
			conn.Close()
		}()
		for msg := range client.send {
			if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		}
	}()

	// Read pump (keep alive, handle client messages)
	go func() {
		defer func() {
			s.wsHub.unregister <- client
			conn.Close()
		}()
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var msg wsMessage
			if err := json.Unmarshal(data, &msg); err != nil {
				continue
			}
			// Handle client->server messages here if needed
			_ = msg
		}
	}()
}
