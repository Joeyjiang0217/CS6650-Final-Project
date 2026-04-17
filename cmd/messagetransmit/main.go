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
	mtclient "github.com/Joeyjiang0217/CS6650-Final-Project/internal/messagetransmit/client"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

type messageTransmitServer struct {
	chatroomv1.UnimplementedMessageTransmitServiceServer
	chatClient           *mtclient.ChatClient
	messageStorageClient *mtclient.MessageStorageClient
}

func newMessageTransmitServer(
	chatClient *mtclient.ChatClient,
	messageStorageClient *mtclient.MessageStorageClient,
) *messageTransmitServer {
	return &messageTransmitServer{
		chatClient:           chatClient,
		messageStorageClient: messageStorageClient,
	}
}

func (s *messageTransmitServer) SendStringMessage(ctx context.Context, req *chatroomv1.SendStringMessageRequest) (*chatroomv1.SendStringMessageResponse, error) {
	senderID := strings.TrimSpace(req.GetSenderId())
	sessionID := strings.TrimSpace(req.GetSessionId())
	content := req.GetContent()

	if senderID == "" {
		return nil, status.Error(codes.InvalidArgument, "sender_id is required")
	}
	if sessionID == "" {
		return nil, status.Error(codes.InvalidArgument, "session_id is required")
	}
	if strings.TrimSpace(content) == "" {
		return nil, status.Error(codes.InvalidArgument, "content is required")
	}

	isMember, err := s.chatClient.IsSessionMember(ctx, sessionID, senderID)
	if err != nil {
		log.Printf("[MessageTransmit] IsSessionMember error: %v", err)
		return nil, status.Error(codes.Internal, "failed to verify session membership")
	}
	if !isMember {
		return nil, status.Error(codes.PermissionDenied, "sender is not a member of the session")
	}

	msg, err := s.messageStorageClient.StoreStringMessage(ctx, sessionID, senderID, content)
	if err != nil {
		log.Printf("[MessageTransmit] StoreStringMessage error: %v", err)
		return nil, status.Error(codes.Internal, "failed to store message")
	}

	// Best effort:
	// Once the sender successfully sends a message, treat that message as read
	// for the sender by advancing last_read_seq to the newly created session_seq.
	if msg != nil {
		if err := s.chatClient.MarkSessionRead(ctx, sessionID, senderID, msg.GetSessionSeq()); err != nil {
			log.Printf("[MessageTransmit] MarkSessionRead after send failed: session_id=%s sender_id=%s seq=%d err=%v",
				sessionID, senderID, msg.GetSessionSeq(), err)
		}
	}

	targetUserIDs, err := s.chatClient.ListSessionMemberIDs(ctx, sessionID)
	if err != nil {
		log.Printf("[MessageTransmit] ListSessionMemberIDs error: %v", err)
		return nil, status.Error(codes.Internal, "failed to list session members")
	}

	return &chatroomv1.SendStringMessageResponse{
		Message:       msg,
		TargetUserIds: targetUserIDs,
	}, nil
}

func main() {
	addr := getEnv("MESSAGETRANSMIT_ADDR", ":50053")
	chatTarget := getEnv("CHATSERVICE_TARGET", "127.0.0.1:50051")
	messageStorageTarget := getEnv("MESSAGESTORAGE_TARGET", "127.0.0.1:50052")
	clientTimeout := time.Duration(getEnvAsInt("MESSAGETRANSMIT_CLIENT_TIMEOUT_SEC", 3)) * time.Second

	chatClient, err := mtclient.NewChatClient(chatTarget, clientTimeout)
	if err != nil {
		log.Fatalf("failed to create chat client: %v", err)
	}
	defer chatClient.Close()

	messageStorageClient, err := mtclient.NewMessageStorageClient(messageStorageTarget, clientTimeout)
	if err != nil {
		log.Fatalf("failed to create message storage client: %v", err)
	}
	defer messageStorageClient.Close()

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("failed to listen on %s: %v", addr, err)
	}

	grpcServer := grpc.NewServer()
	chatroomv1.RegisterMessageTransmitServiceServer(
		grpcServer,
		newMessageTransmitServer(chatClient, messageStorageClient),
	)
	reflection.Register(grpcServer)

	log.Printf("MessageTransmitService listening on %s", addr)
	log.Printf("MessageTransmitService using chatservice=%s messagestorage=%s", chatTarget, messageStorageTarget)

	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("failed to serve MessageTransmitService: %v", err)
		}
	}()

	waitForShutdown(grpcServer)
	log.Println("MessageTransmitService stopped")
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
