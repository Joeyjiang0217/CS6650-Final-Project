package main

import (
	"context"
	"log"
	"time"
)

// startHeartbeat periodically refreshes Redis presence for every user currently
// connected to this instance.
func (s *Server) startHeartbeat() {
	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()

	for range ticker.C {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		users := s.hub.ListUsers()
		for _, userID := range users {
			if err := s.setUserPresence(ctx, userID); err != nil {
				log.Printf("heartbeat refresh failed for %s: %v", userID, err)
			}
		}
		cancel()

		if len(users) > 0 {
			log.Printf("heartbeat refreshed presence for %d users", len(users))
		}
	}
}
