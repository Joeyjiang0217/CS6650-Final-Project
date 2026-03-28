package main

import "time"

// ChatMessage is the canonical message payload used both for real-time delivery
// and for offline persistence in DynamoDB.
type ChatMessage struct {
	MessageID      string    `json:"message_id" dynamodbav:"message_id"`
	ConversationID string    `json:"conversation_id" dynamodbav:"conversation_id"`
	SenderID       string    `json:"sender_id" dynamodbav:"sender_id"`
	Content        string    `json:"content" dynamodbav:"content"`
	MessageType    string    `json:"message_type" dynamodbav:"message_type"`
	Timestamp      time.Time `json:"timestamp" dynamodbav:"timestamp"`
}

// InternalSendRequest is used for instance-to-instance delivery over HTTP.
type InternalSendRequest struct {
	TargetUserID string      `json:"target_user_id"`
	Message      ChatMessage `json:"message"`
}

// WSInboundMessage represents the payload received from a WebSocket client.
type WSInboundMessage struct {
	Type           string `json:"type"`
	ConversationID string `json:"conversation_id"`
	Content        string `json:"content"`
	MessageType    string `json:"message_type"`
}

// WSOutboundMessage wraps events sent back to WebSocket clients.
type WSOutboundMessage struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

// OfflineMessageItem is the DynamoDB representation used for durable offline
// message storage.
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
