package redisx

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type Config struct {
	Addr      string
	Password  string
	DB        int
	KeyTTL    time.Duration
	GatewayID string
}

type Store struct {
	client    *redis.Client
	keyTTL    time.Duration
	gatewayID string
}

func New(cfg Config) (*Store, error) {
	if cfg.Addr == "" {
		cfg.Addr = "127.0.0.1:6379"
	}
	if cfg.KeyTTL <= 0 {
		cfg.KeyTTL = 2 * time.Hour
	}
	if cfg.GatewayID == "" {
		cfg.GatewayID = "gateway-local"
	}

	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("ping redis: %w", err)
	}

	return &Store{
		client:    client,
		keyTTL:    cfg.KeyTTL,
		gatewayID: cfg.GatewayID,
	}, nil
}

func (s *Store) Close() error {
	if s == nil || s.client == nil {
		return nil
	}
	return s.client.Close()
}

func (s *Store) SetOnline(ctx context.Context, userID string) error {
	if userID == "" {
		return fmt.Errorf("userID is required")
	}

	now := time.Now().UTC().Unix()

	pipe := s.client.TxPipeline()
	pipe.Set(ctx, onlineKey(userID), "1", s.keyTTL)
	pipe.Set(ctx, gatewayKey(userID), s.gatewayID, s.keyTTL)
	pipe.Set(ctx, lastSeenKey(userID), now, 0)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("set online state: %w", err)
	}

	return nil
}

func (s *Store) RefreshOnline(ctx context.Context, userID string) error {
	if userID == "" {
		return fmt.Errorf("userID is required")
	}

	now := time.Now().UTC().Unix()

	pipe := s.client.TxPipeline()
	pipe.Expire(ctx, onlineKey(userID), s.keyTTL)
	pipe.Expire(ctx, gatewayKey(userID), s.keyTTL)
	pipe.Set(ctx, lastSeenKey(userID), now, 0)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("refresh online state: %w", err)
	}

	return nil
}

func (s *Store) ClearOnline(ctx context.Context, userID string) error {
	if userID == "" {
		return fmt.Errorf("userID is required")
	}

	now := time.Now().UTC().Unix()

	pipe := s.client.TxPipeline()
	pipe.Del(ctx, onlineKey(userID))
	pipe.Del(ctx, gatewayKey(userID))
	pipe.Set(ctx, lastSeenKey(userID), now, 0)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("clear online state: %w", err)
	}

	return nil
}

func onlineKey(userID string) string {
	return "online:user:" + userID
}

func gatewayKey(userID string) string {
	return "gateway:user:" + userID
}

func lastSeenKey(userID string) string {
	return "lastseen:user:" + userID
}
