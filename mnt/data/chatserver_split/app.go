package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

// run initializes dependencies and starts the HTTP server.
func run() error {
	cfg := loadConfig()
	ctx := context.Background()

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(cfg.AWSRegion))
	if err != nil {
		return err
	}

	ddbClient := dynamodb.NewFromConfig(awsCfg)
	server := NewServer(cfg, ddbClient)

	if err := server.redis.Ping(ctx).Err(); err != nil {
		return err
	}

	go server.startHeartbeat()

	mux := http.NewServeMux()
	mux.HandleFunc("/health", server.handleHealth)
	mux.HandleFunc("/ws", server.handleWebSocket)
	mux.HandleFunc("/internal/send", server.handleInternalSend)

	httpServer := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      loggingMiddleware(mux),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("chat server listening on :%s", cfg.Port)
		log.Printf("instance address: %s", cfg.InstanceAddr)
		log.Printf("redis address: %s", cfg.RedisAddr)
		log.Printf("dynamodb table: %s", cfg.DDBTableName)

		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("listen error: %v", err)
		}
	}()

	waitForShutdown(httpServer)
	return nil
}
