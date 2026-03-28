package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

// handleChatMessage validates an inbound chat event, builds the canonical
// message object, and routes it to every conversation member except the sender.
func (s *Server) handleChatMessage(senderID string, inbound WSInboundMessage) error {
	if inbound.ConversationID == "" {
		return errors.New("missing conversation_id")
	}
	if inbound.Content == "" {
		return errors.New("missing content")
	}

	msgType := inbound.MessageType
	if msgType == "" {
		msgType = "text"
	}

	msg := ChatMessage{
		MessageID:      generateMessageID(),
		ConversationID: inbound.ConversationID,
		SenderID:       senderID,
		Content:        inbound.Content,
		MessageType:    msgType,
		Timestamp:      time.Now().UTC(),
	}

	memberIDs, err := s.getConversationMembers(inbound.ConversationID)
	if err != nil {
		return err
	}

	for _, targetUserID := range memberIDs {
		if targetUserID == senderID {
			continue
		}
		if err := s.routeMessageToUser(targetUserID, msg); err != nil {
			log.Printf("failed to route message to %s: %v", targetUserID, err)
		}
	}

	return nil
}

// routeMessageToUser checks Redis presence and decides whether to deliver the
// message locally, forward it to a remote instance, or persist it offline.
func (s *Server) routeMessageToUser(targetUserID string, msg ChatMessage) error {
	ctx := context.Background()

	presence, err := s.getUserPresence(ctx, targetUserID)
	if err != nil {
		return err
	}

	if presence == "" {
		log.Printf("user %s offline; storing offline message", targetUserID)
		return s.storeOfflineMessage(ctx, targetUserID, msg, "offline_no_presence")
	}

	if normalizeAddr(presence) == normalizeAddr(s.cfg.InstanceAddr) {
		if err := s.sendLocal(targetUserID, msg); err != nil {
			log.Printf("local send failed for %s; storing offline message: %v", targetUserID, err)
			_ = s.removeUserPresence(ctx, targetUserID)
			return s.storeOfflineMessage(ctx, targetUserID, msg, "local_send_failed")
		}
		return nil
	}

	if err := s.sendRemote(presence, targetUserID, msg); err != nil {
		log.Printf("remote send failed for %s; stale presence suspected: %v", targetUserID, err)
		_ = s.removeUserPresence(ctx, targetUserID)
		return s.storeOfflineMessage(ctx, targetUserID, msg, "remote_send_failed")
	}

	return nil
}

// sendLocal writes an outbound event into the local user's channel.
func (s *Server) sendLocal(targetUserID string, msg ChatMessage) error {
	client, ok := s.hub.Get(targetUserID)
	if !ok {
		return errors.New("target user not connected locally")
	}

	outbound := WSOutboundMessage{Type: "chat", Payload: msg}

	data, err := json.Marshal(outbound)
	if err != nil {
		return err
	}

	select {
	case client.Send <- data:
		return nil
	default:
		return errors.New("client send buffer full")
	}
}

// sendRemote forwards a message to another instance using the internal HTTP
// delivery endpoint.
func (s *Server) sendRemote(instanceAddr, targetUserID string, msg ChatMessage) error {
	reqBody := InternalSendRequest{
		TargetUserID: targetUserID,
		Message:      msg,
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	url := strings.TrimRight(instanceAddr, "/") + "/internal/send"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("remote instance returned non-2xx status: %d", resp.StatusCode)
	}

	return nil
}

// getConversationMembers returns demo room membership data.
func (s *Server) getConversationMembers(conversationID string) ([]string, error) {
	memberIDs, ok := s.conversationMembers[conversationID]
	if !ok {
		return nil, errors.New("conversation not found")
	}
	return memberIDs, nil
}
