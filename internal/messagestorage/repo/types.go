package repo

import "time"

type Message struct {
	MessageID  string
	SessionID  string
	SessionSeq uint64
	SenderID   string
	Content    string
	CreatedAt  time.Time
}

type LastMessagePreview struct {
	MessageID  string
	SessionID  string
	SessionSeq uint64
	SenderID   string
	Content    string
	CreatedAt  time.Time
}

type SessionSnapshot struct {
	SessionID   string
	MaxSeq      uint64
	LastMessage *LastMessagePreview
}

type StoreStringMessageInput struct {
	MessageID string
	SessionID string
	SenderID  string
	Content   string
}
