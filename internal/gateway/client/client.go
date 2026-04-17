package client

import (
	"context"
	"fmt"
	"time"

	chatroomv1 "github.com/Joeyjiang0217/CS6650-Final-Project/api/gen/chatroom/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type ChatClient struct {
	conn    *grpc.ClientConn
	client  chatroomv1.ChatServiceClient
	timeout time.Duration
}

func NewChatClient(target string, timeout time.Duration) (*ChatClient, error) {
	if timeout <= 0 {
		timeout = 3 * time.Second
	}

	conn, err := grpc.Dial(
		target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("dial chatservice %s: %w", target, err)
	}

	return &ChatClient{
		conn:    conn,
		client:  chatroomv1.NewChatServiceClient(conn),
		timeout: timeout,
	}, nil
}

func (c *ChatClient) Close() error {
	if c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

func (c *ChatClient) CreateGroupChatSession(
	ctx context.Context,
	creatorID string,
	name string,
	memberIDs []string,
) (*chatroomv1.CreateGroupChatSessionResponse, error) {
	callCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	return c.client.CreateGroupChatSession(callCtx, &chatroomv1.CreateGroupChatSessionRequest{
		CreatorId: creatorID,
		Name:      name,
		MemberIds: memberIDs,
	})
}

func (c *ChatClient) GetChatSessionList(
	ctx context.Context,
	userID string,
) (*chatroomv1.GetChatSessionListResponse, error) {
	callCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	return c.client.GetChatSessionList(callCtx, &chatroomv1.GetChatSessionListRequest{
		UserId: userID,
	})
}

func (c *ChatClient) GetChatSessionMembers(
	ctx context.Context,
	sessionID string,
) (*chatroomv1.GetChatSessionMembersResponse, error) {
	callCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	return c.client.GetChatSessionMembers(callCtx, &chatroomv1.GetChatSessionMembersRequest{
		SessionId: sessionID,
	})
}

func (c *ChatClient) MarkSessionRead(
	ctx context.Context,
	sessionID string,
	userID string,
	lastReadSeq uint64,
) (*chatroomv1.MarkSessionReadResponse, error) {
	callCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	return c.client.MarkSessionRead(callCtx, &chatroomv1.MarkSessionReadRequest{
		SessionId:   sessionID,
		UserId:      userID,
		LastReadSeq: lastReadSeq,
	})
}

type MessageStorageClient struct {
	conn    *grpc.ClientConn
	client  chatroomv1.MessageStorageServiceClient
	timeout time.Duration
}

func NewMessageStorageClient(target string, timeout time.Duration) (*MessageStorageClient, error) {
	if timeout <= 0 {
		timeout = 3 * time.Second
	}

	conn, err := grpc.Dial(
		target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("dial messagestorage %s: %w", target, err)
	}

	return &MessageStorageClient{
		conn:    conn,
		client:  chatroomv1.NewMessageStorageServiceClient(conn),
		timeout: timeout,
	}, nil
}

func (c *MessageStorageClient) Close() error {
	if c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

func (c *MessageStorageClient) GetRecentMessages(
	ctx context.Context,
	sessionID string,
	limit uint32,
) (*chatroomv1.GetRecentMessagesResponse, error) {
	callCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	return c.client.GetRecentMessages(callCtx, &chatroomv1.GetRecentMessagesRequest{
		SessionId: sessionID,
		Limit:     limit,
	})
}

func (c *MessageStorageClient) GetUnreadMessages(
	ctx context.Context,
	sessionID string,
	afterSeq uint64,
	limit uint32,
) (*chatroomv1.GetUnreadMessagesResponse, error) {
	callCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	return c.client.GetUnreadMessages(callCtx, &chatroomv1.GetUnreadMessagesRequest{
		SessionId: sessionID,
		AfterSeq:  afterSeq,
		Limit:     limit,
	})
}

type MessageTransmitClient struct {
	conn    *grpc.ClientConn
	client  chatroomv1.MessageTransmitServiceClient
	timeout time.Duration
}

func NewMessageTransmitClient(target string, timeout time.Duration) (*MessageTransmitClient, error) {
	if timeout <= 0 {
		timeout = 3 * time.Second
	}

	conn, err := grpc.Dial(
		target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("dial messagetransmit %s: %w", target, err)
	}

	return &MessageTransmitClient{
		conn:    conn,
		client:  chatroomv1.NewMessageTransmitServiceClient(conn),
		timeout: timeout,
	}, nil
}

func (c *MessageTransmitClient) Close() error {
	if c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

func (c *MessageTransmitClient) SendStringMessage(
	ctx context.Context,
	senderID string,
	sessionID string,
	content string,
) (*chatroomv1.SendStringMessageResponse, error) {
	callCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	return c.client.SendStringMessage(callCtx, &chatroomv1.SendStringMessageRequest{
		SenderId:  senderID,
		SessionId: sessionID,
		Content:   content,
	})
}
