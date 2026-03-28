package main

import (
	"context"
	"errors"

	"github.com/redis/go-redis/v9"
)

// setUserPresence stores the current instance address under the user's Redis key
// with a TTL so stale entries disappear automatically after failures.
func (s *Server) setUserPresence(ctx context.Context, userID string) error {
	key := "presence:user:" + userID
	return s.redis.Set(ctx, key, s.cfg.InstanceAddr, presenceTTL).Err()
}

// getUserPresence returns the instance currently responsible for the user, or an
// empty string if the user is offline.
func (s *Server) getUserPresence(ctx context.Context, userID string) (string, error) {
	key := "presence:user:" + userID
	val, err := s.redis.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return "", nil
	}
	return val, err
}

// removeUserPresence deletes the user's Redis presence entry.
func (s *Server) removeUserPresence(ctx context.Context, userID string) error {
	key := "presence:user:" + userID
	return s.redis.Del(ctx, key).Err()
}
