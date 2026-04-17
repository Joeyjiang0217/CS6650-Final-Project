package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	chatroomv1 "github.com/Joeyjiang0217/CS6650-Final-Project/api/gen/chatroom/v1"
	chatrepo "github.com/Joeyjiang0217/CS6650-Final-Project/internal/chatservice/repo"
	"github.com/Joeyjiang0217/CS6650-Final-Project/internal/mysqlx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type chatServiceServer struct {
	chatroomv1.UnimplementedChatServiceServer
	repo *chatrepo.Repository
}

func newChatServiceServer(repo *chatrepo.Repository) *chatServiceServer {
	return &chatServiceServer{repo: repo}
}

func (s *chatServiceServer) GetFriendList(ctx context.Context, req *chatroomv1.GetFriendListRequest) (*chatroomv1.GetFriendListResponse, error) {
	userID := strings.TrimSpace(req.GetUserId())
	if userID == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	friends, err := s.repo.GetFriendList(ctx, userID)
	if err != nil {
		log.Printf("[ChatService] GetFriendList repo error: %v", err)
		return nil, status.Error(codes.Internal, "failed to get friend list")
	}

	resp := &chatroomv1.GetFriendListResponse{
		Friends: make([]*chatroomv1.User, 0, len(friends)),
	}
	for _, u := range friends {
		resp.Friends = append(resp.Friends, toProtoUser(u))
	}

	return resp, nil
}

func (s *chatServiceServer) GetChatSessionList(ctx context.Context, req *chatroomv1.GetChatSessionListRequest) (*chatroomv1.GetChatSessionListResponse, error) {
	userID := strings.TrimSpace(req.GetUserId())
	if userID == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	summaries, err := s.repo.GetChatSessionList(ctx, userID)
	if err != nil {
		log.Printf("[ChatService] GetChatSessionList repo error: %v", err)
		return nil, status.Error(codes.Internal, "failed to get chat session list")
	}

	resp := &chatroomv1.GetChatSessionListResponse{
		Sessions: make([]*chatroomv1.ChatSessionSummary, 0, len(summaries)),
	}
	for _, item := range summaries {
		resp.Sessions = append(resp.Sessions, toProtoChatSessionSummary(item))
	}

	return resp, nil
}

func (s *chatServiceServer) CreateGroupChatSession(ctx context.Context, req *chatroomv1.CreateGroupChatSessionRequest) (*chatroomv1.CreateGroupChatSessionResponse, error) {
	creatorID := strings.TrimSpace(req.GetCreatorId())
	name := strings.TrimSpace(req.GetName())

	if creatorID == "" {
		return nil, status.Error(codes.InvalidArgument, "creator_id is required")
	}
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}

	sessionID := fmt.Sprintf("s_%d", time.Now().UnixNano())

	session, members, err := s.repo.CreateGroupChatSession(ctx, chatrepo.CreateGroupChatSessionInput{
		SessionID: sessionID,
		CreatorID: creatorID,
		Name:      name,
		MemberIDs: req.GetMemberIds(),
	})
	if err != nil {
		log.Printf("[ChatService] CreateGroupChatSession repo error: %v", err)
		return nil, status.Error(codes.Internal, "failed to create group chat session")
	}

	resp := &chatroomv1.CreateGroupChatSessionResponse{
		Session: toProtoChatSession(*session),
		Members: make([]*chatroomv1.SessionMember, 0, len(members)),
	}
	for _, m := range members {
		resp.Members = append(resp.Members, toProtoSessionMember(m))
	}

	return resp, nil
}

func (s *chatServiceServer) GetChatSessionMembers(ctx context.Context, req *chatroomv1.GetChatSessionMembersRequest) (*chatroomv1.GetChatSessionMembersResponse, error) {
	sessionID := strings.TrimSpace(req.GetSessionId())
	if sessionID == "" {
		return nil, status.Error(codes.InvalidArgument, "session_id is required")
	}

	members, err := s.repo.GetChatSessionMembers(ctx, sessionID)
	if err != nil {
		log.Printf("[ChatService] GetChatSessionMembers repo error: %v", err)
		return nil, status.Error(codes.Internal, "failed to get session members")
	}

	resp := &chatroomv1.GetChatSessionMembersResponse{
		Members: make([]*chatroomv1.SessionMember, 0, len(members)),
	}
	for _, m := range members {
		resp.Members = append(resp.Members, toProtoSessionMember(m))
	}

	return resp, nil
}

func (s *chatServiceServer) ListSessionMemberIds(ctx context.Context, req *chatroomv1.ListSessionMemberIdsRequest) (*chatroomv1.ListSessionMemberIdsResponse, error) {
	sessionID := strings.TrimSpace(req.GetSessionId())
	if sessionID == "" {
		return nil, status.Error(codes.InvalidArgument, "session_id is required")
	}

	userIDs, err := s.repo.ListSessionMemberIDs(ctx, sessionID)
	if err != nil {
		log.Printf("[ChatService] ListSessionMemberIds repo error: %v", err)
		return nil, status.Error(codes.Internal, "failed to list session member ids")
	}

	return &chatroomv1.ListSessionMemberIdsResponse{
		UserIds: userIDs,
	}, nil
}

func (s *chatServiceServer) IsSessionMember(ctx context.Context, req *chatroomv1.IsSessionMemberRequest) (*chatroomv1.IsSessionMemberResponse, error) {
	sessionID := strings.TrimSpace(req.GetSessionId())
	userID := strings.TrimSpace(req.GetUserId())

	if sessionID == "" || userID == "" {
		return nil, status.Error(codes.InvalidArgument, "session_id and user_id are required")
	}

	ok, err := s.repo.IsSessionMember(ctx, sessionID, userID)
	if err != nil {
		log.Printf("[ChatService] IsSessionMember repo error: %v", err)
		return nil, status.Error(codes.Internal, "failed to check session membership")
	}

	return &chatroomv1.IsSessionMemberResponse{
		IsMember: ok,
	}, nil
}

func (s *chatServiceServer) MarkSessionRead(ctx context.Context, req *chatroomv1.MarkSessionReadRequest) (*chatroomv1.MarkSessionReadResponse, error) {
	sessionID := strings.TrimSpace(req.GetSessionId())
	userID := strings.TrimSpace(req.GetUserId())
	lastReadSeq := req.GetLastReadSeq()

	if sessionID == "" || userID == "" {
		return nil, status.Error(codes.InvalidArgument, "session_id and user_id are required")
	}

	err := s.repo.MarkSessionRead(ctx, sessionID, userID, lastReadSeq)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, status.Error(codes.NotFound, "session member not found")
	}
	if err != nil {
		log.Printf("[ChatService] MarkSessionRead repo error: %v", err)
		return nil, status.Error(codes.Internal, "failed to mark session read")
	}

	return &chatroomv1.MarkSessionReadResponse{
		Success: true,
	}, nil
}

func toProtoUser(u chatrepo.User) *chatroomv1.User {
	return &chatroomv1.User{
		UserId:    u.UserID,
		Nickname:  u.Nickname,
		AvatarUrl: u.AvatarURL,
	}
}

func toProtoChatSession(s chatrepo.ChatSession) *chatroomv1.ChatSession {
	var lastMessageAt *timestamppb.Timestamp
	if s.LastMessageAt != nil {
		lastMessageAt = timestamppb.New(*s.LastMessageAt)
	}

	return &chatroomv1.ChatSession{
		SessionId:     s.SessionID,
		Name:          s.Name,
		Type:          toProtoSessionType(s.Type),
		CreatorId:     s.CreatorID,
		CreatedAt:     timestamppb.New(s.CreatedAt),
		UpdatedAt:     timestamppb.New(s.UpdatedAt),
		LastMessageAt: lastMessageAt,
		LastSeq:       s.LastSeq,
	}
}

func toProtoChatSessionSummary(item chatrepo.ChatSessionSummary) *chatroomv1.ChatSessionSummary {
	return &chatroomv1.ChatSessionSummary{
		Session:     toProtoChatSession(item.Session),
		LastReadSeq: item.LastReadSeq,
		UnreadCount: item.UnreadCount,
	}
}

func toProtoSessionMember(m chatrepo.SessionMember) *chatroomv1.SessionMember {
	return &chatroomv1.SessionMember{
		User:        toProtoUser(m.User),
		Role:        toProtoMemberRole(m.Role),
		JoinedAt:    timestamppb.New(m.JoinedAt),
		LastReadSeq: m.LastReadSeq,
	}
}

func toProtoSessionType(v string) chatroomv1.SessionType {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "direct":
		return chatroomv1.SessionType_SESSION_TYPE_DIRECT
	case "group":
		return chatroomv1.SessionType_SESSION_TYPE_GROUP
	default:
		return chatroomv1.SessionType_SESSION_TYPE_UNSPECIFIED
	}
}

func toProtoMemberRole(v string) chatroomv1.MemberRole {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "owner":
		return chatroomv1.MemberRole_MEMBER_ROLE_OWNER
	case "member":
		return chatroomv1.MemberRole_MEMBER_ROLE_MEMBER
	default:
		return chatroomv1.MemberRole_MEMBER_ROLE_UNSPECIFIED
	}
}

func main() {
	addr := getEnv("CHATSERVICE_ADDR", ":50051")

	db, err := mysqlx.Open(mysqlx.Config{
		User:            getEnv("CHATSERVICE_DB_USER", "root"),
		Password:        getEnv("CHATSERVICE_DB_PASSWORD", "root"),
		Host:            getEnv("CHATSERVICE_DB_HOST", "127.0.0.1"),
		Port:            getEnvAsInt("CHATSERVICE_DB_PORT", 3306),
		DBName:          getEnv("CHATSERVICE_DB_NAME", "chatroom"),
		MaxOpenConns:    getEnvAsInt("CHATSERVICE_DB_MAX_OPEN_CONNS", 20),
		MaxIdleConns:    getEnvAsInt("CHATSERVICE_DB_MAX_IDLE_CONNS", 10),
		ConnMaxLifetime: time.Duration(getEnvAsInt("CHATSERVICE_DB_CONN_MAX_LIFETIME_MIN", 30)) * time.Minute,
		ConnMaxIdleTime: time.Duration(getEnvAsInt("CHATSERVICE_DB_CONN_MAX_IDLE_TIME_MIN", 5)) * time.Minute,
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
	chatroomv1.RegisterChatServiceServer(grpcServer, newChatServiceServer(chatrepo.New(db)))
	reflection.Register(grpcServer)

	log.Printf("ChatService listening on %s", addr)

	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("failed to serve ChatService: %v", err)
		}
	}()

	waitForShutdown(grpcServer)
	log.Println("ChatService stopped")
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
