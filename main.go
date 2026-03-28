package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
)

const (
	presenceTTL       = 60 * time.Second
	heartbeatInterval = 20 * time.Second
	writeWait         = 10 * time.Second
	pongWait          = 60 * time.Second
	pingPeriod        = 25 * time.Second
	maxMessageSize    = 64 * 1024
)

type Config struct {
	Port          string
	InstanceAddr  string
	RedisAddr     string
	RedisPassword string
	RedisDB       int

	AWSRegion    string
	DDBTableName string
}

type ChatMessage struct {
	MessageID      string    `json:"message_id" dynamodbav:"message_id"`
	ConversationID string    `json:"conversation_id" dynamodbav:"conversation_id"`
	SenderID       string    `json:"sender_id" dynamodbav:"sender_id"`
	Content        string    `json:"content" dynamodbav:"content"`
	MessageType    string    `json:"message_type" dynamodbav:"message_type"`
	Timestamp      time.Time `json:"timestamp" dynamodbav:"timestamp"`
}

type InternalSendRequest struct {
	TargetUserID string      `json:"target_user_id"`
	Message      ChatMessage `json:"message"`
}

type WSInboundMessage struct {
	Type           string `json:"type"`
	ConversationID string `json:"conversation_id"`
	Content        string `json:"content"`
	MessageType    string `json:"message_type"`
}

type WSOutboundMessage struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

type OfflineMessageItem struct {
	ReceiverID     string    `dynamodbav:"receiver_id"`
	SortKey        string    `dynamodbav:"sort_key"`
	MessageID      string    `dynamodbav:"message_id"`
	ConversationID string    `dynamodbav:"conversation_id"`
	SenderID       string    `dynamodbav:"sender_id"`
	Content        string    `dynamodbav:"content"`
	MessageType    string    `dynamodbav:"message_type"`
	Timestamp      time.Time `dynamodbav:"timestamp"`
	CreatedAt      time.Time `dynamodbav:"created_at"`
	DeliveryReason string    `dynamodbav:"delivery_reason"`
}

type Client struct {
	UserID string
	Conn   *websocket.Conn
	Send   chan []byte
}

type Hub struct {
	mu      sync.RWMutex
	clients map[string]*Client
}

func NewHub() *Hub {
	return &Hub{
		clients: make(map[string]*Client),
	}
}

func (h *Hub) Add(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[c.UserID] = c
}

func (h *Hub) Remove(userID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.clients, userID)
}

func (h *Hub) Get(userID string) (*Client, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	c, ok := h.clients[userID]
	return c, ok
}

func (h *Hub) ListUsers() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	users := make([]string, 0, len(h.clients))
	for userID := range h.clients {
		users = append(users, userID)
	}
	return users
}

type Server struct {
	cfg        Config
	hub        *Hub
	redis      *redis.Client
	ddb        *dynamodb.Client
	httpClient *http.Client
	upgrader   websocket.Upgrader

	// mock conversation membership for now
	conversationMembers map[string][]string
}

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

func main() {
	cfg := Config{
		Port:          getEnv("PORT", "8080"),
		RedisAddr:     getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),
		RedisDB:       0,
		AWSRegion:     getEnv("AWS_REGION", "us-west-2"),
		DDBTableName:  getEnv("DDB_TABLE_NAME", "chat-offline-messages"),
	}

	instanceAddr := strings.TrimSpace(os.Getenv("INSTANCE_ADDR"))
	if instanceAddr == "" {
		instanceAddr = "http://" + getLocalIP() + ":" + cfg.Port
	}
	cfg.InstanceAddr = instanceAddr

	ctx := context.Background()

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(cfg.AWSRegion))
	if err != nil {
		log.Fatalf("failed to load aws config: %v", err)
	}

	ddbClient := dynamodb.NewFromConfig(awsCfg)
	server := NewServer(cfg, ddbClient)

	if err := server.redis.Ping(ctx).Err(); err != nil {
		log.Fatalf("redis connect failed: %v", err)
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
}

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

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":        "ok",
		"instance_addr": s.cfg.InstanceAddr,
	})
}

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
		if err := s.deliverOfflineMessages(context.Background(), userID); err != nil {
			log.Printf("deliver offline messages failed for %s: %v", userID, err)
		}
	}()
}

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

func (s *Server) handleChatMessage(senderID string, inbound WSInboundMessage) error {
	if inbound.ConversationID == "" {
		return errors.New("missing conversation_id")
	}
	if inbound.Content == "" {
		return errors.New("missing content")
	}

	msgType := inbound.MessageType
	if msgType == "" {
		msgType = "text"
	}

	msg := ChatMessage{
		MessageID:      generateMessageID(),
		ConversationID: inbound.ConversationID,
		SenderID:       senderID,
		Content:        inbound.Content,
		MessageType:    msgType,
		Timestamp:      time.Now().UTC(),
	}

	memberIDs, err := s.getConversationMembers(inbound.ConversationID)
	if err != nil {
		return err
	}

	for _, targetUserID := range memberIDs {
		if targetUserID == senderID {
			continue
		}
		if err := s.routeMessageToUser(targetUserID, msg); err != nil {
			log.Printf("failed to route message to %s: %v", targetUserID, err)
		}
	}

	return nil
}

func (s *Server) routeMessageToUser(targetUserID string, msg ChatMessage) error {
	ctx := context.Background()

	presence, err := s.getUserPresence(ctx, targetUserID)
	if err != nil {
		return err
	}

	if presence == "" {
		log.Printf("user %s offline; storing offline message", targetUserID)
		return s.storeOfflineMessage(ctx, targetUserID, msg, "offline_no_presence")
	}

	if normalizeAddr(presence) == normalizeAddr(s.cfg.InstanceAddr) {
		if err := s.sendLocal(targetUserID, msg); err != nil {
			log.Printf("local send failed for %s; storing offline message: %v", targetUserID, err)
			_ = s.removeUserPresence(ctx, targetUserID)
			return s.storeOfflineMessage(ctx, targetUserID, msg, "local_send_failed")
		}
		return nil
	}

	if err := s.sendRemote(presence, targetUserID, msg); err != nil {
		log.Printf("remote send failed for %s; stale presence suspected: %v", targetUserID, err)
		_ = s.removeUserPresence(ctx, targetUserID)
		return s.storeOfflineMessage(ctx, targetUserID, msg, "remote_send_failed")
	}

	return nil
}

func (s *Server) sendLocal(targetUserID string, msg ChatMessage) error {
	client, ok := s.hub.Get(targetUserID)
	if !ok {
		return errors.New("target user not connected locally")
	}

	outbound := WSOutboundMessage{
		Type:    "chat",
		Payload: msg,
	}

	data, err := json.Marshal(outbound)
	if err != nil {
		return err
	}

	select {
	case client.Send <- data:
		return nil
	default:
		return errors.New("client send buffer full")
	}
}

func (s *Server) sendRemote(instanceAddr, targetUserID string, msg ChatMessage) error {
	reqBody := InternalSendRequest{
		TargetUserID: targetUserID,
		Message:      msg,
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	url := strings.TrimRight(instanceAddr, "/") + "/internal/send"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("remote instance returned non-2xx status: %d", resp.StatusCode)
	}

	return nil
}

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

	writeJSON(w, http.StatusOK, map[string]string{
		"status": "delivered",
	})
}

func (s *Server) getConversationMembers(conversationID string) ([]string, error) {
	memberIDs, ok := s.conversationMembers[conversationID]
	if !ok {
		return nil, errors.New("conversation not found")
	}
	return memberIDs, nil
}

func (s *Server) setUserPresence(ctx context.Context, userID string) error {
	key := "presence:user:" + userID
	return s.redis.Set(ctx, key, s.cfg.InstanceAddr, presenceTTL).Err()
}

func (s *Server) getUserPresence(ctx context.Context, userID string) (string, error) {
	key := "presence:user:" + userID
	val, err := s.redis.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return "", nil
	}
	return val, err
}

func (s *Server) removeUserPresence(ctx context.Context, userID string) error {
	key := "presence:user:" + userID
	return s.redis.Del(ctx, key).Err()
}

func (s *Server) storeOfflineMessage(ctx context.Context, receiverID string, msg ChatMessage, reason string) error {
	item := OfflineMessageItem{
		ReceiverID:     receiverID,
		SortKey:        buildOfflineSortKey(msg.Timestamp, msg.MessageID),
		MessageID:      msg.MessageID,
		ConversationID: msg.ConversationID,
		SenderID:       msg.SenderID,
		Content:        msg.Content,
		MessageType:    msg.MessageType,
		Timestamp:      msg.Timestamp,
		CreatedAt:      time.Now().UTC(),
		DeliveryReason: reason,
	}

	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		return err
	}

	_, err = s.ddb.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: &s.cfg.DDBTableName,
		Item:      av,
	})
	return err
}

func (s *Server) listOfflineMessages(ctx context.Context, receiverID string) ([]OfflineMessageItem, error) {
	keyCondition := "receiver_id = :rid"

	out, err := s.ddb.Query(ctx, &dynamodb.QueryInput{
		TableName:              &s.cfg.DDBTableName,
		KeyConditionExpression: &keyCondition,
		ExpressionAttributeValues: map[string]ddbtypes.AttributeValue{
			":rid": &ddbtypes.AttributeValueMemberS{Value: receiverID},
		},
	})
	if err != nil {
		return nil, err
	}

	var items []OfflineMessageItem
	if err := attributevalue.UnmarshalListOfMaps(out.Items, &items); err != nil {
		return nil, err
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].SortKey < items[j].SortKey
	})

	return items, nil
}

func (s *Server) deleteOfflineMessage(ctx context.Context, receiverID, sortKey string) error {
	key, err := attributevalue.MarshalMap(map[string]string{
		"receiver_id": receiverID,
		"sort_key":    sortKey,
	})
	if err != nil {
		return err
	}

	_, err = s.ddb.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: &s.cfg.DDBTableName,
		Key:       key,
	})
	return err
}

func (s *Server) deliverOfflineMessages(ctx context.Context, userID string) error {
	items, err := s.listOfflineMessages(ctx, userID)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		return nil
	}

	log.Printf("delivering %d offline messages to %s", len(items), userID)

	for _, item := range items {
		msg := ChatMessage{
			MessageID:      item.MessageID,
			ConversationID: item.ConversationID,
			SenderID:       item.SenderID,
			Content:        item.Content,
			MessageType:    item.MessageType,
			Timestamp:      item.Timestamp,
		}

		if err := s.sendLocal(userID, msg); err != nil {
			return fmt.Errorf("failed while replaying offline message %s: %w", item.MessageID, err)
		}

		if err := s.deleteOfflineMessage(ctx, userID, item.SortKey); err != nil {
			return fmt.Errorf("failed deleting offline message %s: %w", item.MessageID, err)
		}
	}

	return nil
}

func buildOfflineSortKey(ts time.Time, messageID string) string {
	return ts.UTC().Format(time.RFC3339Nano) + "#" + messageID
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}

func waitForShutdown(srv *http.Server) {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	log.Println("shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
}

func getEnv(key, fallback string) string {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return fallback
	}
	return val
}

func normalizeAddr(addr string) string {
	return strings.TrimRight(strings.TrimSpace(addr), "/")
}

func generateMessageID() string {
	return time.Now().UTC().Format("20060102150405.000000000")
}

func getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1"
	}

	for _, addr := range addrs {
		ipnet, ok := addr.(*net.IPNet)
		if !ok || ipnet.IP.IsLoopback() {
			continue
		}
		if ip := ipnet.IP.To4(); ip != nil {
			return ip.String()
		}
	}

	return "127.0.0.1"
}
