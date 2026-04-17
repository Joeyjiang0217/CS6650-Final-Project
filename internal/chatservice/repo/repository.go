package repo

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type Repository struct {
	db *sql.DB
}

func New(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) GetFriendList(ctx context.Context, userID string) ([]User, error) {
	const q = `
SELECT
	u.user_id,
	u.nickname,
	COALESCE(u.avatar_url, '')
FROM relations rel
JOIN users u ON u.user_id = rel.friend_id
WHERE rel.user_id = ?
ORDER BY u.nickname ASC, u.user_id ASC
`

	rows, err := r.db.QueryContext(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("query friend list: %w", err)
	}
	defer rows.Close()

	var out []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.UserID, &u.Nickname, &u.AvatarURL); err != nil {
			return nil, fmt.Errorf("scan friend list row: %w", err)
		}
		out = append(out, u)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate friend list rows: %w", err)
	}

	return out, nil
}

func (r *Repository) GetChatSessionList(ctx context.Context, userID string) ([]ChatSessionSummary, error) {
	const q = `
SELECT
	s.session_id,
	s.name,
	s.type,
	s.creator_id,
	s.created_at,
	s.updated_at,
	s.last_message_at,
	s.last_seq,
	m.last_read_seq
FROM chat_session_members m
JOIN chat_sessions s ON s.session_id = m.session_id
WHERE m.user_id = ?
ORDER BY COALESCE(s.last_message_at, s.created_at) DESC, s.session_id ASC
`

	rows, err := r.db.QueryContext(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("query chat session list: %w", err)
	}
	defer rows.Close()

	var out []ChatSessionSummary
	for rows.Next() {
		var s ChatSession
		var lastMessageAt sql.NullTime
		var lastReadSeq uint64

		if err := rows.Scan(
			&s.SessionID,
			&s.Name,
			&s.Type,
			&s.CreatorID,
			&s.CreatedAt,
			&s.UpdatedAt,
			&lastMessageAt,
			&s.LastSeq,
			&lastReadSeq,
		); err != nil {
			return nil, fmt.Errorf("scan chat session row: %w", err)
		}

		if lastMessageAt.Valid {
			t := lastMessageAt.Time
			s.LastMessageAt = &t
		}

		unread := uint64(0)
		if s.LastSeq > lastReadSeq {
			unread = s.LastSeq - lastReadSeq
		}

		out = append(out, ChatSessionSummary{
			Session:     s,
			LastReadSeq: lastReadSeq,
			UnreadCount: unread,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate chat session rows: %w", err)
	}

	return out, nil
}

func (r *Repository) CreateGroupChatSession(ctx context.Context, in CreateGroupChatSessionInput) (*ChatSession, []SessionMember, error) {
	memberIDs := normalizeMemberIDs(in.CreatorID, in.MemberIDs)
	if in.SessionID == "" {
		return nil, nil, fmt.Errorf("session_id is required")
	}
	if in.CreatorID == "" {
		return nil, nil, fmt.Errorf("creator_id is required")
	}
	if in.Name == "" {
		return nil, nil, fmt.Errorf("name is required")
	}
	if len(memberIDs) == 0 {
		return nil, nil, fmt.Errorf("at least one member is required")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	now := time.Now().UTC()

	const insertSession = `
INSERT INTO chat_sessions (
	session_id, name, type, creator_id, last_seq, created_at, updated_at, last_message_at
) VALUES (?, ?, 'group', ?, 0, ?, ?, NULL)
`
	if _, err := tx.ExecContext(ctx, insertSession, in.SessionID, in.Name, in.CreatorID, now, now); err != nil {
		return nil, nil, fmt.Errorf("insert chat session: %w", err)
	}

	const insertMember = `
INSERT INTO chat_session_members (
	session_id, user_id, role, joined_at, last_read_seq
) VALUES (?, ?, ?, ?, 0)
`
	for _, uid := range memberIDs {
		role := "member"
		if uid == in.CreatorID {
			role = "owner"
		}
		if _, err := tx.ExecContext(ctx, insertMember, in.SessionID, uid, role, now); err != nil {
			return nil, nil, fmt.Errorf("insert chat session member %s: %w", uid, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, nil, fmt.Errorf("commit tx: %w", err)
	}

	session := &ChatSession{
		SessionID:     in.SessionID,
		Name:          in.Name,
		Type:          "group",
		CreatorID:     in.CreatorID,
		CreatedAt:     now,
		UpdatedAt:     now,
		LastMessageAt: nil,
		LastSeq:       0,
	}

	members, err := r.GetChatSessionMembers(ctx, in.SessionID)
	if err != nil {
		return session, nil, err
	}

	return session, members, nil
}

func (r *Repository) GetChatSessionMembers(ctx context.Context, sessionID string) ([]SessionMember, error) {
	const q = `
SELECT
	m.user_id,
	COALESCE(u.nickname, ''),
	COALESCE(u.avatar_url, ''),
	m.role,
	m.joined_at,
	m.last_read_seq
FROM chat_session_members m
LEFT JOIN users u ON u.user_id = m.user_id
WHERE m.session_id = ?
ORDER BY
	CASE m.role WHEN 'owner' THEN 0 ELSE 1 END,
	m.joined_at ASC,
	m.user_id ASC
`

	rows, err := r.db.QueryContext(ctx, q, sessionID)
	if err != nil {
		return nil, fmt.Errorf("query session members: %w", err)
	}
	defer rows.Close()

	var out []SessionMember
	for rows.Next() {
		var m SessionMember
		if err := rows.Scan(
			&m.User.UserID,
			&m.User.Nickname,
			&m.User.AvatarURL,
			&m.Role,
			&m.JoinedAt,
			&m.LastReadSeq,
		); err != nil {
			return nil, fmt.Errorf("scan session member row: %w", err)
		}
		out = append(out, m)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate session member rows: %w", err)
	}

	return out, nil
}

func (r *Repository) ListSessionMemberIDs(ctx context.Context, sessionID string) ([]string, error) {
	const q = `
SELECT user_id
FROM chat_session_members
WHERE session_id = ?
ORDER BY user_id ASC
`

	rows, err := r.db.QueryContext(ctx, q, sessionID)
	if err != nil {
		return nil, fmt.Errorf("query session member ids: %w", err)
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var userID string
		if err := rows.Scan(&userID); err != nil {
			return nil, fmt.Errorf("scan session member id row: %w", err)
		}
		out = append(out, userID)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate session member id rows: %w", err)
	}

	return out, nil
}

func (r *Repository) IsSessionMember(ctx context.Context, sessionID, userID string) (bool, error) {
	const q = `
SELECT 1
FROM chat_session_members
WHERE session_id = ? AND user_id = ?
LIMIT 1
`

	var one int
	err := r.db.QueryRowContext(ctx, q, sessionID, userID).Scan(&one)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("query is session member: %w", err)
	}

	return true, nil
}

func (r *Repository) MarkSessionRead(ctx context.Context, sessionID, userID string, lastReadSeq uint64) error {
	const q = `
UPDATE chat_session_members
SET last_read_seq = GREATEST(last_read_seq, ?)
WHERE session_id = ? AND user_id = ?
`

	res, err := r.db.ExecContext(ctx, q, lastReadSeq, sessionID, userID)
	if err != nil {
		return fmt.Errorf("update last_read_seq: %w", err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if affected == 0 {
		return sql.ErrNoRows
	}

	return nil
}

func normalizeMemberIDs(creatorID string, memberIDs []string) []string {
	seen := make(map[string]struct{}, len(memberIDs)+1)
	out := make([]string, 0, len(memberIDs)+1)

	add := func(id string) {
		id = strings.TrimSpace(id)
		if id == "" {
			return
		}
		if _, ok := seen[id]; ok {
			return
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}

	add(creatorID)
	for _, id := range memberIDs {
		add(id)
	}

	return out
}
