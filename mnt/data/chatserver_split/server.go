package main

import (
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
)

// Server owns the shared dependencies and runtime state for one chat instance.
type Server struct {
	cfg        Config
	hub        *Hub
	redis      *redis.Client
	ddb        *dynamodb.Client
	httpClient *http.Client
	upgrader   websocket.Upgrader

	// conversationMembers is a temporary in-memory membership source for demo
	// purposes. In a real system, this would come from a database or service.
	conversationMembers map[string][]string
}

// NewServer builds a server and wires up all dependencies.
func NewServer(cfg Config, ddbClient *dynamodb.Client) *Server {
	return &Server{
		cfg: cfg,
		hub: NewHub(),
		redis: redis.NewClient(&redis.Options{
			Addr:     cfg.RedisAddr,
			Password: cfg.RedisPassword,
			DB:       cfg.RedisDB,
		}),
		ddb: ddbClient,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
		conversationMembers: map[string][]string{
			"room-1": {"user1", "user2", "user3"},
			"room-2": {"user2", "user4"},
		},
	}
}
