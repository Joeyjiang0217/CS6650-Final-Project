package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	chatroomv1 "github.com/Joeyjiang0217/CS6650-Final-Project/api/gen/chatroom/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	chatAddr := flag.String("chat", "127.0.0.1:50051", "chatservice gRPC address")
	transmitAddr := flag.String("transmit", "127.0.0.1:50053", "messagetransmit gRPC address")
	creatorID := flag.String("creator", "u1", "group creator user_id")
	groupName := flag.String("name", "demo-group", "group name")
	membersRaw := flag.String("members", "u1,u2,u3", "comma-separated member ids")
	content := flag.String("content", "hello from testclient", "message content")

	flag.Parse()

	memberIDs := parseMemberIDs(*membersRaw)
	if len(memberIDs) == 0 {
		log.Fatal("members cannot be empty")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	chatConn, err := grpc.Dial(
		*chatAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatalf("failed to connect chatservice: %v", err)
	}
	defer chatConn.Close()

	transmitConn, err := grpc.Dial(
		*transmitAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatalf("failed to connect messagetransmit: %v", err)
	}
	defer transmitConn.Close()

	chatClient := chatroomv1.NewChatServiceClient(chatConn)
	transmitClient := chatroomv1.NewMessageTransmitServiceClient(transmitConn)

	fmt.Println("== Step 1: CreateGroupChatSession ==")
	createResp, err := chatClient.CreateGroupChatSession(ctx, &chatroomv1.CreateGroupChatSessionRequest{
		CreatorId: *creatorID,
		Name:      *groupName,
		MemberIds: memberIDs,
	})
	if err != nil {
		log.Fatalf("CreateGroupChatSession failed: %v", err)
	}

	session := createResp.GetSession()
	if session == nil {
		log.Fatal("CreateGroupChatSession returned nil session")
	}

	fmt.Printf("Created session:\n")
	fmt.Printf("  session_id: %s\n", session.GetSessionId())
	fmt.Printf("  name:       %s\n", session.GetName())
	fmt.Printf("  type:       %s\n", session.GetType().String())
	fmt.Printf("  creator_id: %s\n", session.GetCreatorId())
	fmt.Printf("Members:\n")
	for _, m := range createResp.GetMembers() {
		if m.GetUser() == nil {
			continue
		}
		fmt.Printf("  - user_id=%s role=%s last_read_seq=%d\n",
			m.GetUser().GetUserId(),
			m.GetRole().String(),
			m.GetLastReadSeq(),
		)
	}

	fmt.Println()
	fmt.Println("== Step 2: SendStringMessage ==")
	sendResp, err := transmitClient.SendStringMessage(ctx, &chatroomv1.SendStringMessageRequest{
		SenderId:  *creatorID,
		SessionId: session.GetSessionId(),
		Content:   *content,
	})
	if err != nil {
		log.Fatalf("SendStringMessage failed: %v", err)
	}

	msg := sendResp.GetMessage()
	if msg == nil {
		log.Fatal("SendStringMessage returned nil message")
	}

	fmt.Printf("Stored message:\n")
	fmt.Printf("  message_id:  %s\n", msg.GetMessageId())
	fmt.Printf("  session_id:  %s\n", msg.GetSessionId())
	fmt.Printf("  session_seq: %d\n", msg.GetSessionSeq())
	fmt.Printf("  sender_id:   %s\n", msg.GetSenderId())
	fmt.Printf("  content:     %s\n", msg.GetContent())
	if msg.GetCreatedAt() != nil {
		fmt.Printf("  created_at:  %s\n", msg.GetCreatedAt().AsTime().Format(time.RFC3339Nano))
	}

	fmt.Printf("Target user ids: %v\n", sendResp.GetTargetUserIds())

	fmt.Println()
	fmt.Println("Integration test finished successfully.")
}

func parseMemberIDs(raw string) []string {
	parts := strings.Split(raw, ",")
	seen := make(map[string]struct{}, len(parts))
	out := make([]string, 0, len(parts))

	for _, p := range parts {
		id := strings.TrimSpace(p)
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
