package repo

import "time"

type User struct {
	UserID    string
	Nickname  string
	AvatarURL string
}

type ChatSession struct {
	SessionID     string
	Name          string
	Type          string
	CreatorID     string
	CreatedAt     time.Time
	UpdatedAt     time.Time
	LastMessageAt *time.Time
	LastSeq       uint64
}

type ChatSessionSummary struct {
	Session     ChatSession
	LastReadSeq uint64
	UnreadCount uint64
}

type SessionMember struct {
	User        User
	Role        string
	JoinedAt    time.Time
	LastReadSeq uint64
}

type CreateGroupChatSessionInput struct {
	SessionID string
	CreatorID string
	Name      string
	MemberIDs []string
}
