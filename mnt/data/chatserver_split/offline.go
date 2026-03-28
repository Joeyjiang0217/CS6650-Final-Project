package main

import (
	"context"
	"fmt"
	"log"
	"sort"
	"time"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// storeOfflineMessage writes an undelivered message to DynamoDB so it can be
// replayed when the receiver reconnects.
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

// listOfflineMessages fetches all pending messages for a receiver and sorts them
// by the DynamoDB sort key.
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

// deleteOfflineMessage removes a replayed message from DynamoDB after successful
// delivery.
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

// deliverOfflineMessages replays pending messages to a user after they connect.
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

// buildOfflineSortKey ensures messages can be replayed in chronological order.
func buildOfflineSortKey(ts time.Time, messageID string) string {
	return ts.UTC().Format(time.RFC3339Nano) + "#" + messageID
}
