package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	chatroomv1 "github.com/Joeyjiang0217/CS6650-Final-Project/api/gen/chatroom/v1"
	msgrepo "github.com/Joeyjiang0217/CS6650-Final-Project/internal/messagestorage/repo"
	"github.com/Joeyjiang0217/CS6650-Final-Project/internal/mysqlx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type messageStorageServer struct {
	chatroomv1.UnimplementedMessageStorageServiceServer
	repo *msgrepo.Repository
}

func newMessageStorageServer(repo *msgrepo.Repository) *messageStorageServer {
	return &messageStorageServer{repo: repo}
}

func (s *messageStorageServer) StoreStringMessage(ctx context.Context, req *chatroomv1.StoreStringMessageRequest) (*chatroomv1.StoreStringMessageResponse, error) {
	sessionID := strings.TrimSpace(req.GetSessionId())
	senderID := strings.TrimSpace(req.GetSenderId())
	content := req.GetContent()

	if sessionID == "" || senderID == "" {
		return nil, status.Error(codes.InvalidArgument, "session_id and sender_id are required")
	}
	if strings.TrimSpace(content) == "" {
		return nil, status.Error(codes.InvalidArgument, "content is required")
	}

	messageID := generateMessageID()

	msg, err := s.repo.StoreStringMessage(ctx, msgrepo.StoreStringMessageInput{
		MessageID: messageID,
		SessionID: sessionID,
		SenderID:  senderID,
		Content:   content,
	})
	if err != nil {
		log.Printf("[MessageStorage] StoreStringMessage repo error: %v", err)
		return nil, status.Error(codes.Internal, "failed to store message")
	}

	return &chatroomv1.StoreStringMessageResponse{
		Message: toProtoMessage(*msg),
	}, nil
}

func (s *messageStorageServer) GetRecentMessages(ctx context.Context, req *chatroomv1.GetRecentMessagesRequest) (*chatroomv1.GetRecentMessagesResponse, error) {
	sessionID := strings.TrimSpace(req.GetSessionId())
	if sessionID == "" {
		return nil, status.Error(codes.InvalidArgument, "session_id is required")
	}

	items, err := s.repo.GetRecentMessages(ctx, sessionID, req.GetLimit())
	if err != nil {
		log.Printf("[MessageStorage] GetRecentMessages repo error: %v", err)
		return nil, status.Error(codes.Internal, "failed to get recent messages")
	}

	return &chatroomv1.GetRecentMessagesResponse{
		Messages: toProtoMessages(items),
	}, nil
}

func (s *messageStorageServer) GetHistoryMessages(ctx context.Context, req *chatroomv1.GetHistoryMessagesRequest) (*chatroomv1.GetHistoryMessagesResponse, error) {
	sessionID := strings.TrimSpace(req.GetSessionId())
	if sessionID == "" {
		return nil, status.Error(codes.InvalidArgument, "session_id is required")
	}

	items, err := s.repo.GetHistoryMessages(ctx, sessionID, req.GetBeforeSeq(), req.GetLimit())
	if err != nil {
		log.Printf("[MessageStorage] GetHistoryMessages repo error: %v", err)
		return nil, status.Error(codes.Internal, "failed to get history messages")
	}

	return &chatroomv1.GetHistoryMessagesResponse{
		Messages: toProtoMessages(items),
	}, nil
}

func (s *messageStorageServer) GetUnreadMessages(ctx context.Context, req *chatroomv1.GetUnreadMessagesRequest) (*chatroomv1.GetUnreadMessagesResponse, error) {
	sessionID := strings.TrimSpace(req.GetSessionId())
	if sessionID == "" {
		return nil, status.Error(codes.InvalidArgument, "session_id is required")
	}

	items, err := s.repo.GetUnreadMessages(ctx, sessionID, req.GetAfterSeq(), req.GetLimit())
	if err != nil {
		log.Printf("[MessageStorage] GetUnreadMessages repo error: %v", err)
		return nil, status.Error(codes.Internal, "failed to get unread messages")
	}

	return &chatroomv1.GetUnreadMessagesResponse{
		Messages: toProtoMessages(items),
	}, nil
}

func (s *messageStorageServer) GetSessionLastMessage(ctx context.Context, req *chatroomv1.GetSessionLastMessageRequest) (*chatroomv1.GetSessionLastMessageResponse, error) {
	sessionID := strings.TrimSpace(req.GetSessionId())
	if sessionID == "" {
		return nil, status.Error(codes.InvalidArgument, "session_id is required")
	}

	lastMsg, err := s.repo.GetSessionLastMessage(ctx, sessionID)
	if err != nil {
		log.Printf("[MessageStorage] GetSessionLastMessage repo error: %v", err)
		return nil, status.Error(codes.Internal, "failed to get session last message")
	}

	return &chatroomv1.GetSessionLastMessageResponse{
		LastMessage: toProtoLastMessagePreview(lastMsg),
	}, nil
}

func (s *messageStorageServer) BatchGetSessionSnapshots(ctx context.Context, req *chatroomv1.BatchGetSessionSnapshotsRequest) (*chatroomv1.BatchGetSessionSnapshotsResponse, error) {
	items, err := s.repo.BatchGetSessionSnapshots(ctx, req.GetSessionIds())
	if err != nil {
		log.Printf("[MessageStorage] BatchGetSessionSnapshots repo error: %v", err)
		return nil, status.Error(codes.Internal, "failed to get session snapshots")
	}

	resp := &chatroomv1.BatchGetSessionSnapshotsResponse{
		Snapshots: make([]*chatroomv1.SessionSnapshot, 0, len(items)),
	}
	for _, item := range items {
		resp.Snapshots = append(resp.Snapshots, &chatroomv1.SessionSnapshot{
			SessionId:   item.SessionID,
			MaxSeq:      item.MaxSeq,
			LastMessage: toProtoLastMessagePreview(item.LastMessage),
		})
	}

	return resp, nil
}

func toProtoMessage(m msgrepo.Message) *chatroomv1.Message {
	return &chatroomv1.Message{
		MessageId:  m.MessageID,
		SessionId:  m.SessionID,
		SessionSeq: m.SessionSeq,
		SenderId:   m.SenderID,
		Content:    m.Content,
		CreatedAt:  timestamppb.New(m.CreatedAt),
	}
}

func toProtoMessages(items []msgrepo.Message) []*chatroomv1.Message {
	out := make([]*chatroomv1.Message, 0, len(items))
	for _, item := range items {
		out = append(out, toProtoMessage(item))
	}
	return out
}

func toProtoLastMessagePreview(m *msgrepo.LastMessagePreview) *chatroomv1.LastMessagePreview {
	if m == nil {
		return nil
	}
	return &chatroomv1.LastMessagePreview{
		MessageId:  m.MessageID,
		SessionId:  m.SessionID,
		SessionSeq: m.SessionSeq,
		SenderId:   m.SenderID,
		Content:    m.Content,
		CreatedAt:  timestamppb.New(m.CreatedAt),
	}
}

func generateMessageID() string {
	return "m_" + strconv.FormatInt(time.Now().UnixNano(), 10)
}

func main() {
	addr := getEnv("MESSAGESTORAGE_ADDR", ":50052")

	db, err := mysqlx.Open(mysqlx.Config{
		User:            getEnv("MESSAGESTORAGE_DB_USER", "root"),
		Password:        getEnv("MESSAGESTORAGE_DB_PASSWORD", "root"),
		Host:            getEnv("MESSAGESTORAGE_DB_HOST", "127.0.0.1"),
		Port:            getEnvAsInt("MESSAGESTORAGE_DB_PORT", 3306),
		DBName:          getEnv("MESSAGESTORAGE_DB_NAME", "chatroom"),
		MaxOpenConns:    getEnvAsInt("MESSAGESTORAGE_DB_MAX_OPEN_CONNS", 20),
		MaxIdleConns:    getEnvAsInt("MESSAGESTORAGE_DB_MAX_IDLE_CONNS", 10),
		ConnMaxLifetime: time.Duration(getEnvAsInt("MESSAGESTORAGE_DB_CONN_MAX_LIFETIME_MIN", 30)) * time.Minute,
		ConnMaxIdleTime: time.Duration(getEnvAsInt("MESSAGESTORAGE_DB_CONN_MAX_IDLE_TIME_MIN", 5)) * time.Minute,
	})
	if err != nil {
		log.Fatalf("failed to connect mysql: %v", err)
	}
	defer db.Close()

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("failed to listen on %s: %v", addr, err)
	}

	grpcServer := grpc.NewServer()
	chatroomv1.RegisterMessageStorageServiceServer(grpcServer, newMessageStorageServer(msgrepo.New(db)))
	reflection.Register(grpcServer)

	log.Printf("MessageStorageService listening on %s", addr)

	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("failed to serve MessageStorageService: %v", err)
		}
	}()

	waitForShutdown(grpcServer)
	log.Println("MessageStorageService stopped")
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

func waitForShutdown(server *grpc.Server) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	log.Println("shutdown signal received")
	server.GracefulStop()
}
