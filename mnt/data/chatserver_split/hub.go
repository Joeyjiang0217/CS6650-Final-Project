package main

import (
	"sync"

	"github.com/gorilla/websocket"
)

// Client represents one connected WebSocket user.
type Client struct {
	UserID string
	Conn   *websocket.Conn
	Send   chan []byte
}

// Hub stores all clients connected to the current server instance.
type Hub struct {
	mu      sync.RWMutex
	clients map[string]*Client
}

// NewHub creates an empty in-memory client registry.
func NewHub() *Hub {
	return &Hub{clients: make(map[string]*Client)}
}

// Add registers a connected client.
func (h *Hub) Add(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[c.UserID] = c
}

// Remove unregisters a client by user ID.
func (h *Hub) Remove(userID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.clients, userID)
}

// Get returns the connected client for a user if present.
func (h *Hub) Get(userID string) (*Client, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	c, ok := h.clients[userID]
	return c, ok
}

// ListUsers returns all currently connected user IDs.
func (h *Hub) ListUsers() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	users := make([]string, 0, len(h.clients))
	for userID := range h.clients {
		users = append(users, userID)
	}
	return users
}
