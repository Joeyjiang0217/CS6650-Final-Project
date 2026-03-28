package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
)

// handleHealth exposes a simple readiness endpoint.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":        "ok",
		"instance_addr": s.cfg.InstanceAddr,
	})
}

// handleWebSocket upgrades the request, registers the client locally, writes
// presence to Redis, and starts the read/write loops.
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	userID := strings.TrimSpace(r.URL.Query().Get("user_id"))
	if userID == "" {
		http.Error(w, "missing user_id", http.StatusBadRequest)
		return
	}

	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("websocket upgrade failed: %v", err)
		return
	}

	client := &Client{
		UserID: userID,
		Conn:   conn,
		Send:   make(chan []byte, 256),
	}

	s.hub.Add(client)

	if err := s.setUserPresence(r.Context(), userID); err != nil {
		log.Printf("failed to set user presence for %s: %v", userID, err)
	}

	log.Printf("user connected: %s", userID)

	go s.writePump(client)
	go s.readPump(client)

	go func() {
		if err := s.deliverOfflineMessages(r.Context(), userID); err != nil {
			log.Printf("deliver offline messages failed for %s: %v", userID, err)
		}
	}()
}

// handleInternalSend is called by another instance when it needs this server to
// deliver a message to a locally connected user.
func (s *Server) handleInternalSend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req InternalSendRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.TargetUserID == "" {
		http.Error(w, "missing target_user_id", http.StatusBadRequest)
		return
	}

	if err := s.sendLocal(req.TargetUserID, req.Message); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "delivered"})
}
