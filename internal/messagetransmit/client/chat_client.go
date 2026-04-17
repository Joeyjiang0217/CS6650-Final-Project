package client

import (
	"context"
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
		return nil, err
	}

	return &ChatClient{
		conn:    conn,
		client:  chatroomv1.NewChatServiceClient(conn),
		timeout: timeout,
	}, nil
}

func (c *ChatClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *ChatClient) IsSessionMember(ctx context.Context, sessionID, userID string) (bool, error) {
	reqCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	resp, err := c.client.IsSessionMember(reqCtx, &chatroomv1.IsSessionMemberRequest{
		SessionId: sessionID,
		UserId:    userID,
	})
	if err != nil {
		return false, err
	}

	return resp.GetIsMember(), nil
}

func (c *ChatClient) ListSessionMemberIDs(ctx context.Context, sessionID string) ([]string, error) {
	reqCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	resp, err := c.client.ListSessionMemberIds(reqCtx, &chatroomv1.ListSessionMemberIdsRequest{
		SessionId: sessionID,
	})
	if err != nil {
		return nil, err
	}

	return resp.GetUserIds(), nil
}

func (c *ChatClient) MarkSessionRead(ctx context.Context, sessionID, userID string, lastReadSeq uint64) error {
	reqCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	_, err := c.client.MarkSessionRead(reqCtx, &chatroomv1.MarkSessionReadRequest{
		SessionId:   sessionID,
		UserId:      userID,
		LastReadSeq: lastReadSeq,
	})
	return err
}
