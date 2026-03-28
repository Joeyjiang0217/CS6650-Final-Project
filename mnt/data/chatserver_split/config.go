package main

import (
	"strings"
	"time"
)

const (
	// presenceTTL controls how long a user's Redis presence entry stays valid
	// unless it is refreshed by the heartbeat loop.
	presenceTTL = 60 * time.Second

	// heartbeatInterval controls how often the server refreshes presence for
	// users connected to this instance.
	heartbeatInterval = 20 * time.Second

	// WebSocket timing limits.
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = 25 * time.Second
	maxMessageSize = 64 * 1024
)

// Config contains runtime configuration for the chat server.
type Config struct {
	Port          string
	InstanceAddr  string
	RedisAddr     string
	RedisPassword string
	RedisDB       int

	AWSRegion    string
	DDBTableName string
}

// loadConfig reads environment variables and builds the server config.
func loadConfig() Config {
	cfg := Config{
		Port:          getEnv("PORT", "8080"),
		RedisAddr:     getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),
		RedisDB:       0,
		AWSRegion:     getEnv("AWS_REGION", "us-west-2"),
		DDBTableName:  getEnv("DDB_TABLE_NAME", "chat-offline-messages"),
	}

	instanceAddr := strings.TrimSpace(getEnv("INSTANCE_ADDR", ""))
	if instanceAddr == "" {
		instanceAddr = "http://" + getLocalIP() + ":" + cfg.Port
	}
	cfg.InstanceAddr = instanceAddr

	return cfg
}
