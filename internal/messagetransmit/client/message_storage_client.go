package client

import (
	"context"
	"time"

	chatroomv1 "github.com/Joeyjiang0217/CS6650-Final-Project/api/gen/chatroom/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

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
		return nil, err
	}

	return &MessageStorageClient{
		conn:    conn,
		client:  chatroomv1.NewMessageStorageServiceClient(conn),
		timeout: timeout,
	}, nil
}

func (c *MessageStorageClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *MessageStorageClient) StoreStringMessage(ctx context.Context, sessionID, senderID, content string) (*chatroomv1.Message, error) {
	reqCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	resp, err := c.client.StoreStringMessage(reqCtx, &chatroomv1.StoreStringMessageRequest{
		SessionId: sessionID,
		SenderId:  senderID,
		Content:   content,
	})
	if err != nil {
		return nil, err
	}

	return resp.GetMessage(), nil
}
