package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	chatroomv1 "github.com/Joeyjiang0217/CS6650-Final-Project/api/gen/chatroom/v1"
	gwclient "github.com/Joeyjiang0217/CS6650-Final-Project/internal/gateway/client"
	"github.com/Joeyjiang0217/CS6650-Final-Project/internal/redisx"
	"github.com/gorilla/websocket"
)

type gatewayServer struct {
	chatClient            *gwclient.ChatClient
	messageStorageClient  *gwclient.MessageStorageClient
	messageTransmitClient *gwclient.MessageTransmitClient
	redisStore            *redisx.Store
	wsManager             *wsManager
	upgrader              websocket.Upgrader
}

type createGroupSessionRequest struct {
	CreatorID string   `json:"creator_id"`
	Name      string   `json:"name"`
	MemberIDs []string `json:"member_ids"`
}

type sendMessageRequest struct {
	SenderID  string `json:"sender_id"`
	SessionID string `json:"session_id"`
	Content   string `json:"content"`
}

type markReadRequest struct {
	UserID      string `json:"user_id"`
	LastReadSeq uint64 `json:"last_read_seq"`
}

type userResponse struct {
	UserID    string `json:"user_id"`
	Nickname  string `json:"nickname,omitempty"`
	AvatarURL string `json:"avatar_url,omitempty"`
}

type sessionMemberResponse struct {
	User        *userResponse `json:"user"`
	Role        string        `json:"role"`
	JoinedAt    string        `json:"joined_at,omitempty"`
	LastReadSeq uint64        `json:"last_read_seq"`
}

type chatSessionResponse struct {
	SessionID     string `json:"session_id"`
	Name          string `json:"name"`
	Type          string `json:"type"`
	CreatorID     string `json:"creator_id"`
	CreatedAt     string `json:"created_at,omitempty"`
	UpdatedAt     string `json:"updated_at,omitempty"`
	LastMessageAt string `json:"last_message_at,omitempty"`
	LastSeq       uint64 `json:"last_seq"`
}

type chatSessionSummaryResponse struct {
	Session     *chatSessionResponse `json:"session"`
	LastReadSeq uint64               `json:"last_read_seq"`
	UnreadCount uint64               `json:"unread_count"`
}

type messageResponse struct {
	MessageID  string `json:"message_id"`
	SessionID  string `json:"session_id"`
	SessionSeq uint64 `json:"session_seq"`
	SenderID   string `json:"sender_id"`
	Content    string `json:"content"`
	CreatedAt  string `json:"created_at,omitempty"`
}

type wsClient struct {
	conn   *websocket.Conn
	userID string
	mu     sync.Mutex
}

type wsManager struct {
	mu    sync.RWMutex
	users map[string]map[*wsClient]struct{}
}

type wsIncomingMessage struct {
	Type   string `json:"type"`
	UserID string `json:"user_id,omitempty"`
}

type wsOutgoingMessage struct {
	Type      string           `json:"type"`
	UserID    string           `json:"user_id,omitempty"`
	SessionID string           `json:"session_id,omitempty"`
	Message   *messageResponse `json:"message,omitempty"`
	Error     string           `json:"error,omitempty"`
}

func newWSManager() *wsManager {
	return &wsManager{
		users: make(map[string]map[*wsClient]struct{}),
	}
}

func (m *wsManager) bind(client *wsClient, userID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if client.userID != "" && client.userID != userID {
		if set, ok := m.users[client.userID]; ok {
			delete(set, client)
			if len(set) == 0 {
				delete(m.users, client.userID)
			}
		}
	}

	client.userID = userID
	if _, ok := m.users[userID]; !ok {
		m.users[userID] = make(map[*wsClient]struct{})
	}
	m.users[userID][client] = struct{}{}
}

func (m *wsManager) remove(client *wsClient) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if client.userID == "" {
		return
	}

	if set, ok := m.users[client.userID]; ok {
		delete(set, client)
		if len(set) == 0 {
			delete(m.users, client.userID)
		}
	}
	client.userID = ""
}

func (m *wsManager) snapshotClients(userIDs []string) []*wsClient {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var out []*wsClient
	seen := make(map[*wsClient]struct{})

	for _, userID := range userIDs {
		if set, ok := m.users[userID]; ok {
			for c := range set {
				if _, exists := seen[c]; exists {
					continue
				}
				seen[c] = struct{}{}
				out = append(out, c)
			}
		}
	}

	return out
}

func (m *wsManager) pushNewMessage(userIDs []string, msg *chatroomv1.Message) {
	clients := m.snapshotClients(userIDs)
	if len(clients) == 0 || msg == nil {
		return
	}

	payload := wsOutgoingMessage{
		Type:      "new_message",
		SessionID: msg.GetSessionId(),
		Message:   protoToMessage(msg),
	}

	for _, client := range clients {
		if err := client.writeJSON(payload); err != nil {
			log.Printf("[Gateway][WS] push new_message error user_id=%s: %v", client.userID, err)
			_ = client.close()
			m.remove(client)
		}
	}
}

func (c *wsClient) writeJSON(v any) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return c.conn.WriteJSON(v)
}

func (c *wsClient) close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn.Close()
}

func newGatewayServer(
	chatClient *gwclient.ChatClient,
	messageStorageClient *gwclient.MessageStorageClient,
	messageTransmitClient *gwclient.MessageTransmitClient,
	redisStore *redisx.Store,
) *gatewayServer {
	return &gatewayServer{
		chatClient:            chatClient,
		messageStorageClient:  messageStorageClient,
		messageTransmitClient: messageTransmitClient,
		redisStore:            redisStore,
		wsManager:             newWSManager(),
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
	}
}

func (g *gatewayServer) routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", g.handleHealth)
	mux.HandleFunc("/healthz", g.handleHealth)
	mux.HandleFunc("/ws", g.handleWebSocket)

	mux.HandleFunc("/api/sessions", g.handleListSessions)
	mux.HandleFunc("/api/sessions/group", g.handleCreateGroupSession)
	mux.HandleFunc("/api/messages/send", g.handleSendMessage)
	mux.HandleFunc("/api/sessions/", g.handleSessionSubroutes)

	return loggingMiddleware(corsMiddleware(mux))
}

func (g *gatewayServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
	})
}

func (g *gatewayServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := g.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[Gateway][WS] upgrade error: %v", err)
		return
	}

	client := &wsClient{conn: conn}
	log.Printf("[Gateway][WS] client connected from %s", r.RemoteAddr)

	defer func() {
		userID := client.userID
		if userID != "" && g.redisStore != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			if err := g.redisStore.ClearOnline(ctx, userID); err != nil {
				log.Printf("[Gateway][Redis] clear online failed user_id=%s: %v", userID, err)
			}
			cancel()
		}

		g.wsManager.remove(client)
		_ = client.close()
		log.Printf("[Gateway][WS] client disconnected user_id=%s", userID)
	}()

	_ = conn.SetReadDeadline(time.Time{})
	conn.SetPongHandler(func(string) error {
		_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))

		if client.userID != "" && g.redisStore != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			if err := g.redisStore.RefreshOnline(ctx, client.userID); err != nil {
				log.Printf("[Gateway][Redis] refresh online failed user_id=%s: %v", client.userID, err)
			}
			cancel()
		}
		return nil
	})

	for {
		var msg wsIncomingMessage
		if err := conn.ReadJSON(&msg); err != nil {
			log.Printf("[Gateway][WS] read error: %v", err)
			return
		}

		switch strings.TrimSpace(msg.Type) {
		case "auth":
			userID := strings.TrimSpace(msg.UserID)
			if userID == "" {
				_ = client.writeJSON(wsOutgoingMessage{
					Type:  "error",
					Error: "user_id is required",
				})
				continue
			}

			g.wsManager.bind(client, userID)

			if g.redisStore != nil {
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				if err := g.redisStore.SetOnline(ctx, userID); err != nil {
					log.Printf("[Gateway][Redis] set online failed user_id=%s: %v", userID, err)
				}
				cancel()
			}

			if err := client.writeJSON(wsOutgoingMessage{
				Type:   "auth_ok",
				UserID: userID,
			}); err != nil {
				log.Printf("[Gateway][WS] auth_ok write error: %v", err)
				return
			}

			log.Printf("[Gateway][WS] bound connection to user_id=%s", userID)

		case "ping":
			if client.userID != "" && g.redisStore != nil {
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				if err := g.redisStore.RefreshOnline(ctx, client.userID); err != nil {
					log.Printf("[Gateway][Redis] refresh online failed user_id=%s: %v", client.userID, err)
				}
				cancel()
			}

			if err := client.writeJSON(wsOutgoingMessage{Type: "pong"}); err != nil {
				log.Printf("[Gateway][WS] pong write error: %v", err)
				return
			}

		default:
			_ = client.writeJSON(wsOutgoingMessage{
				Type:  "error",
				Error: "unsupported message type",
			})
		}
	}
}

func (g *gatewayServer) handleListSessions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}

	userID := strings.TrimSpace(r.URL.Query().Get("user_id"))
	if userID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": "user_id is required",
		})
		return
	}

	resp, err := g.chatClient.GetChatSessionList(r.Context(), userID)
	if err != nil {
		log.Printf("[Gateway] GetChatSessionList error: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error": "failed to get session list",
		})
		return
	}

	out := make([]*chatSessionSummaryResponse, 0, len(resp.GetSessions()))
	for _, item := range resp.GetSessions() {
		if item == nil {
			continue
		}
		out = append(out, protoToSessionSummary(item))
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"user_id":  userID,
		"sessions": out,
	})
}

func (g *gatewayServer) handleCreateGroupSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}

	var req createGroupSessionRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": err.Error(),
		})
		return
	}

	req.CreatorID = strings.TrimSpace(req.CreatorID)
	req.Name = strings.TrimSpace(req.Name)
	req.MemberIDs = normalizeIDs(req.MemberIDs)

	if req.CreatorID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "creator_id is required"})
		return
	}
	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "name is required"})
		return
	}

	resp, err := g.chatClient.CreateGroupChatSession(r.Context(), req.CreatorID, req.Name, req.MemberIDs)
	if err != nil {
		log.Printf("[Gateway] CreateGroupChatSession error: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error": "failed to create group session",
		})
		return
	}

	out := map[string]any{
		"session": protoToSession(resp.GetSession()),
		"members": protoToMembers(resp.GetMembers()),
	}
	writeJSON(w, http.StatusOK, out)
}

func (g *gatewayServer) handleSendMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}

	var req sendMessageRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": err.Error(),
		})
		return
	}

	req.SenderID = strings.TrimSpace(req.SenderID)
	req.SessionID = strings.TrimSpace(req.SessionID)
	req.Content = strings.TrimSpace(req.Content)

	if req.SenderID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "sender_id is required"})
		return
	}
	if req.SessionID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "session_id is required"})
		return
	}
	if req.Content == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "content is required"})
		return
	}

	resp, err := g.messageTransmitClient.SendStringMessage(r.Context(), req.SenderID, req.SessionID, req.Content)
	if err != nil {
		log.Printf("[Gateway] SendStringMessage error: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error": "failed to send message",
		})
		return
	}

	if resp.GetMessage() != nil {
		g.wsManager.pushNewMessage(resp.GetTargetUserIds(), resp.GetMessage())
	}

	out := map[string]any{
		"message":         protoToMessage(resp.GetMessage()),
		"target_user_ids": resp.GetTargetUserIds(),
	}
	writeJSON(w, http.StatusOK, out)
}

func (g *gatewayServer) handleSessionSubroutes(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/sessions/")
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")

	if len(parts) < 2 {
		writeJSON(w, http.StatusNotFound, map[string]any{
			"error": "route not found",
		})
		return
	}

	sessionID := strings.TrimSpace(parts[0])
	if sessionID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": "session_id is required",
		})
		return
	}

	if len(parts) == 2 && parts[1] == "members" {
		g.handleGetSessionMembers(w, r, sessionID)
		return
	}

	if len(parts) == 2 && parts[1] == "read" {
		g.handleMarkSessionRead(w, r, sessionID)
		return
	}

	if len(parts) == 3 && parts[1] == "messages" && parts[2] == "recent" {
		g.handleGetRecentMessages(w, r, sessionID)
		return
	}

	if len(parts) == 3 && parts[1] == "messages" && parts[2] == "unread" {
		g.handleGetUnreadMessages(w, r, sessionID)
		return
	}

	writeJSON(w, http.StatusNotFound, map[string]any{
		"error": "route not found",
	})
}

func (g *gatewayServer) handleGetSessionMembers(w http.ResponseWriter, r *http.Request, sessionID string) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}

	resp, err := g.chatClient.GetChatSessionMembers(r.Context(), sessionID)
	if err != nil {
		log.Printf("[Gateway] GetChatSessionMembers error: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error": "failed to get session members",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"session_id": sessionID,
		"members":    protoToMembers(resp.GetMembers()),
	})
}

func (g *gatewayServer) handleMarkSessionRead(w http.ResponseWriter, r *http.Request, sessionID string) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}

	var req markReadRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": err.Error(),
		})
		return
	}

	req.UserID = strings.TrimSpace(req.UserID)
	if req.UserID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": "user_id is required",
		})
		return
	}

	resp, err := g.chatClient.MarkSessionRead(r.Context(), sessionID, req.UserID, req.LastReadSeq)
	if err != nil {
		log.Printf("[Gateway] MarkSessionRead error: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error": "failed to mark session read",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"session_id":    sessionID,
		"user_id":       req.UserID,
		"last_read_seq": req.LastReadSeq,
		"success":       resp.GetSuccess(),
	})
}

func (g *gatewayServer) handleGetRecentMessages(w http.ResponseWriter, r *http.Request, sessionID string) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}

	limit := uint32(20)
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil || v <= 0 {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"error": "limit must be a positive integer",
			})
			return
		}
		limit = uint32(v)
	}

	resp, err := g.messageStorageClient.GetRecentMessages(r.Context(), sessionID, limit)
	if err != nil {
		log.Printf("[Gateway] GetRecentMessages error: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error": "failed to get recent messages",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"session_id": sessionID,
		"messages":   protoToMessages(resp.GetMessages()),
	})
}

func (g *gatewayServer) handleGetUnreadMessages(w http.ResponseWriter, r *http.Request, sessionID string) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}

	afterSeq := uint64(0)
	if raw := strings.TrimSpace(r.URL.Query().Get("after_seq")); raw != "" {
		v, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"error": "after_seq must be a non-negative integer",
			})
			return
		}
		afterSeq = v
	}

	limit := uint32(50)
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil || v <= 0 {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"error": "limit must be a positive integer",
			})
			return
		}
		limit = uint32(v)
	}

	resp, err := g.messageStorageClient.GetUnreadMessages(r.Context(), sessionID, afterSeq, limit)
	if err != nil {
		log.Printf("[Gateway] GetUnreadMessages error: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error": "failed to get unread messages",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"session_id": sessionID,
		"after_seq":  afterSeq,
		"messages":   protoToMessages(resp.GetMessages()),
	})
}

func protoToUser(u *chatroomv1.User) *userResponse {
	if u == nil {
		return nil
	}
	return &userResponse{
		UserID:    u.GetUserId(),
		Nickname:  u.GetNickname(),
		AvatarURL: u.GetAvatarUrl(),
	}
}

func protoToMembers(items []*chatroomv1.SessionMember) []*sessionMemberResponse {
	out := make([]*sessionMemberResponse, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}

		joinedAt := ""
		if item.GetJoinedAt() != nil {
			joinedAt = item.GetJoinedAt().AsTime().Format(time.RFC3339Nano)
		}

		out = append(out, &sessionMemberResponse{
			User:        protoToUser(item.GetUser()),
			Role:        item.GetRole().String(),
			JoinedAt:    joinedAt,
			LastReadSeq: item.GetLastReadSeq(),
		})
	}
	return out
}

func protoToSession(s *chatroomv1.ChatSession) *chatSessionResponse {
	if s == nil {
		return nil
	}

	createdAt := ""
	updatedAt := ""
	lastMessageAt := ""

	if s.GetCreatedAt() != nil {
		createdAt = s.GetCreatedAt().AsTime().Format(time.RFC3339Nano)
	}
	if s.GetUpdatedAt() != nil {
		updatedAt = s.GetUpdatedAt().AsTime().Format(time.RFC3339Nano)
	}
	if s.GetLastMessageAt() != nil {
		lastMessageAt = s.GetLastMessageAt().AsTime().Format(time.RFC3339Nano)
	}

	return &chatSessionResponse{
		SessionID:     s.GetSessionId(),
		Name:          s.GetName(),
		Type:          s.GetType().String(),
		CreatorID:     s.GetCreatorId(),
		CreatedAt:     createdAt,
		UpdatedAt:     updatedAt,
		LastMessageAt: lastMessageAt,
		LastSeq:       s.GetLastSeq(),
	}
}

func protoToSessionSummary(item *chatroomv1.ChatSessionSummary) *chatSessionSummaryResponse {
	if item == nil {
		return nil
	}

	return &chatSessionSummaryResponse{
		Session:     protoToSession(item.GetSession()),
		LastReadSeq: item.GetLastReadSeq(),
		UnreadCount: item.GetUnreadCount(),
	}
}

func protoToMessage(m *chatroomv1.Message) *messageResponse {
	if m == nil {
		return nil
	}

	createdAt := ""
	if m.GetCreatedAt() != nil {
		createdAt = m.GetCreatedAt().AsTime().Format(time.RFC3339Nano)
	}

	return &messageResponse{
		MessageID:  m.GetMessageId(),
		SessionID:  m.GetSessionId(),
		SessionSeq: m.GetSessionSeq(),
		SenderID:   m.GetSenderId(),
		Content:    m.GetContent(),
		CreatedAt:  createdAt,
	}
}

func protoToMessages(items []*chatroomv1.Message) []*messageResponse {
	out := make([]*messageResponse, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		out = append(out, protoToMessage(item))
	}
	return out
}

func decodeJSONBody(r *http.Request, dst any) error {
	defer r.Body.Close()

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	return decoder.Decode(dst)
}

func normalizeIDs(ids []string) []string {
	seen := make(map[string]struct{}, len(ids))
	out := make([]string, 0, len(ids))

	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}

	return out
}

func writeMethodNotAllowed(w http.ResponseWriter) {
	writeJSON(w, http.StatusMethodNotAllowed, map[string]any{
		"error": "method not allowed",
	})
}

func writeJSON(w http.ResponseWriter, statusCode int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("[Gateway] writeJSON encode error: %v", err)
	}
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Max-Age", "86400")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("[Gateway] %s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}

func main() {
	addr := getEnv("GATEWAY_ADDR", ":8080")
	chatTarget := getEnv("CHATSERVICE_TARGET", "127.0.0.1:50051")
	messageStorageTarget := getEnv("MESSAGESTORAGE_TARGET", "127.0.0.1:50052")
	messageTransmitTarget := getEnv("MESSAGETRANSMIT_TARGET", "127.0.0.1:50053")
	clientTimeout := time.Duration(getEnvAsInt("GATEWAY_CLIENT_TIMEOUT_SEC", 3)) * time.Second
	gatewayID := getEnv("GATEWAY_INSTANCE_ID", defaultGatewayInstanceID())

	redisStore, err := redisx.New(redisx.Config{
		Addr:      getEnv("REDIS_ADDR", "127.0.0.1:6379"),
		Password:  getEnv("REDIS_PASSWORD", ""),
		DB:        getEnvAsInt("REDIS_DB", 0),
		KeyTTL:    time.Duration(getEnvAsInt("REDIS_ONLINE_TTL_SEC", 7200)) * time.Second,
		GatewayID: gatewayID,
	})
	if err != nil {
		log.Fatalf("failed to create redis store: %v", err)
	}
	defer redisStore.Close()

	chatClient, err := gwclient.NewChatClient(chatTarget, clientTimeout)
	if err != nil {
		log.Fatalf("failed to create chat client: %v", err)
	}
	defer chatClient.Close()

	messageStorageClient, err := gwclient.NewMessageStorageClient(messageStorageTarget, clientTimeout)
	if err != nil {
		log.Fatalf("failed to create message storage client: %v", err)
	}
	defer messageStorageClient.Close()

	messageTransmitClient, err := gwclient.NewMessageTransmitClient(messageTransmitTarget, clientTimeout)
	if err != nil {
		log.Fatalf("failed to create message transmit client: %v", err)
	}
	defer messageTransmitClient.Close()

	server := newGatewayServer(chatClient, messageStorageClient, messageTransmitClient, redisStore)

	httpServer := &http.Server{
		Addr:         addr,
		Handler:      server.routes(),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 20 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("Gateway listening on %s", addr)
		log.Printf("Gateway targets: chat=%s storage=%s transmit=%s redis=%s gateway_id=%s",
			chatTarget,
			messageStorageTarget,
			messageTransmitTarget,
			getEnv("REDIS_ADDR", "127.0.0.1:6379"),
			gatewayID,
		)

		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("gateway server error: %v", err)
		}
	}()

	waitForShutdown(httpServer)
	log.Println("Gateway stopped")
}

func waitForShutdown(server *http.Server) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch

	log.Println("shutdown signal received")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("gateway graceful shutdown error: %v", err)
	}
}

func getEnv(key, fallback string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	return v
}

func getEnvAsInt(key string, fallback int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}

	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func defaultGatewayInstanceID() string {
	hostname, err := os.Hostname()
	if err != nil || strings.TrimSpace(hostname) == "" {
		return "gateway-local"
	}
	return "gateway-" + hostname
}
