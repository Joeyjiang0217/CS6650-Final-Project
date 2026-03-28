package main

import (
	"context"

	"encoding/json"
	"log"
	"time"

	"github.com/gorilla/websocket"
)

// readPump continuously reads frames from one WebSocket client and forwards
// valid chat events into the server's routing logic.
func (s *Server) readPump(c *Client) {
	defer func() {
		s.cleanupClient(c)
	}()

	c.Conn.SetReadLimit(maxMessageSize)
	_ = c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	c.Conn.SetPongHandler(func(string) error {
		return c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		_, messageBytes, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("websocket read error for %s: %v", c.UserID, err)
			}
			break
		}

		var inbound WSInboundMessage
		if err := json.Unmarshal(messageBytes, &inbound); err != nil {
			log.Printf("invalid inbound message from %s: %v", c.UserID, err)
			continue
		}

		switch inbound.Type {
		case "chat":
			if err := s.handleChatMessage(c.UserID, inbound); err != nil {
				log.Printf("handle chat message error: %v", err)
			}
		default:
			log.Printf("unknown message type from %s: %s", c.UserID, inbound.Type)
		}
	}
}

// writePump sends queued outbound messages and periodic ping frames to keep the
// WebSocket connection healthy.
func (s *Server) writePump(c *Client) {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.Conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.Send:
			_ = c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.Conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}

		case <-ticker.C:
			_ = c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// cleanupClient removes local client state and deletes the user's presence key
// so other instances stop routing traffic here.
func (s *Server) cleanupClient(c *Client) {
	s.hub.Remove(c.UserID)
	close(c.Send)
	_ = c.Conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := s.removeUserPresence(ctx, c.UserID); err != nil {
		log.Printf("failed to remove redis presence for %s: %v", c.UserID, err)
	}

	log.Printf("user disconnected: %s", c.UserID)
}
