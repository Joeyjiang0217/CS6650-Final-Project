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

func (r *Repository) StoreStringMessage(ctx context.Context, in StoreStringMessageInput) (*Message, error) {
	if in.MessageID == "" {
		return nil, fmt.Errorf("message_id is required")
	}
	if in.SessionID == "" {
		return nil, fmt.Errorf("session_id is required")
	}
	if in.SenderID == "" {
		return nil, fmt.Errorf("sender_id is required")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	var lastSeq uint64
	const selectSessionForUpdate = `
SELECT last_seq
FROM chat_sessions
WHERE session_id = ?
FOR UPDATE
`
	if err := tx.QueryRowContext(ctx, selectSessionForUpdate, in.SessionID).Scan(&lastSeq); err != nil {
		return nil, fmt.Errorf("select session for update: %w", err)
	}

	newSeq := lastSeq + 1
	now := time.Now().UTC()

	const updateSession = `
UPDATE chat_sessions
SET last_seq = ?, last_message_at = ?, updated_at = ?
WHERE session_id = ?
`
	if _, err := tx.ExecContext(ctx, updateSession, newSeq, now, now, in.SessionID); err != nil {
		return nil, fmt.Errorf("update chat session seq: %w", err)
	}

	const insertMessage = `
INSERT INTO messages (
	message_id, session_id, session_seq, sender_id, content, created_at
) VALUES (?, ?, ?, ?, ?, ?)
`
	if _, err := tx.ExecContext(ctx, insertMessage, in.MessageID, in.SessionID, newSeq, in.SenderID, in.Content, now); err != nil {
		return nil, fmt.Errorf("insert message: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}

	return &Message{
		MessageID:  in.MessageID,
		SessionID:  in.SessionID,
		SessionSeq: newSeq,
		SenderID:   in.SenderID,
		Content:    in.Content,
		CreatedAt:  now,
	}, nil
}

func (r *Repository) GetRecentMessages(ctx context.Context, sessionID string, limit uint32) ([]Message, error) {
	if limit == 0 {
		limit = 20
	}

	const q = `
SELECT
	message_id,
	session_id,
	session_seq,
	sender_id,
	content,
	created_at
FROM messages
WHERE session_id = ?
ORDER BY session_seq DESC
LIMIT ?
`

	rows, err := r.db.QueryContext(ctx, q, sessionID, limit)
	if err != nil {
		return nil, fmt.Errorf("query recent messages: %w", err)
	}
	defer rows.Close()

	out, err := scanMessages(rows)
	if err != nil {
		return nil, err
	}

	reverseMessages(out)
	return out, nil
}

func (r *Repository) GetHistoryMessages(ctx context.Context, sessionID string, beforeSeq uint64, limit uint32) ([]Message, error) {
	if limit == 0 {
		limit = 20
	}

	if beforeSeq == 0 {
		return r.GetRecentMessages(ctx, sessionID, limit)
	}

	const q = `
SELECT
	message_id,
	session_id,
	session_seq,
	sender_id,
	content,
	created_at
FROM messages
WHERE session_id = ?
  AND session_seq < ?
ORDER BY session_seq DESC
LIMIT ?
`

	rows, err := r.db.QueryContext(ctx, q, sessionID, beforeSeq, limit)
	if err != nil {
		return nil, fmt.Errorf("query history messages: %w", err)
	}
	defer rows.Close()

	out, err := scanMessages(rows)
	if err != nil {
		return nil, err
	}

	reverseMessages(out)
	return out, nil
}

func (r *Repository) GetUnreadMessages(ctx context.Context, sessionID string, afterSeq uint64, limit uint32) ([]Message, error) {
	if limit == 0 {
		limit = 50
	}

	const q = `
SELECT
	message_id,
	session_id,
	session_seq,
	sender_id,
	content,
	created_at
FROM messages
WHERE session_id = ?
  AND session_seq > ?
ORDER BY session_seq ASC
LIMIT ?
`

	rows, err := r.db.QueryContext(ctx, q, sessionID, afterSeq, limit)
	if err != nil {
		return nil, fmt.Errorf("query unread messages: %w", err)
	}
	defer rows.Close()

	return scanMessages(rows)
}

func (r *Repository) GetSessionLastMessage(ctx context.Context, sessionID string) (*LastMessagePreview, error) {
	const q = `
SELECT
	message_id,
	session_id,
	session_seq,
	sender_id,
	content,
	created_at
FROM messages
WHERE session_id = ?
ORDER BY session_seq DESC
LIMIT 1
`

	var m LastMessagePreview
	err := r.db.QueryRowContext(ctx, q, sessionID).Scan(
		&m.MessageID,
		&m.SessionID,
		&m.SessionSeq,
		&m.SenderID,
		&m.Content,
		&m.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query last message: %w", err)
	}

	return &m, nil
}

func (r *Repository) BatchGetSessionSnapshots(ctx context.Context, sessionIDs []string) ([]SessionSnapshot, error) {
	sessionIDs = normalizeIDs(sessionIDs)
	if len(sessionIDs) == 0 {
		return []SessionSnapshot{}, nil
	}

	q := fmt.Sprintf(`
SELECT session_id, last_seq
FROM chat_sessions
WHERE session_id IN (%s)
`, placeholders(len(sessionIDs)))

	args := make([]any, 0, len(sessionIDs))
	for _, id := range sessionIDs {
		args = append(args, id)
	}

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("query session snapshots: %w", err)
	}
	defer rows.Close()

	maxSeqMap := make(map[string]uint64, len(sessionIDs))
	for rows.Next() {
		var sessionID string
		var maxSeq uint64
		if err := rows.Scan(&sessionID, &maxSeq); err != nil {
			return nil, fmt.Errorf("scan session snapshot row: %w", err)
		}
		maxSeqMap[sessionID] = maxSeq
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate session snapshot rows: %w", err)
	}

	out := make([]SessionSnapshot, 0, len(sessionIDs))
	for _, sessionID := range sessionIDs {
		s := SessionSnapshot{
			SessionID: sessionID,
			MaxSeq:    maxSeqMap[sessionID],
		}

		lastMsg, err := r.GetSessionLastMessage(ctx, sessionID)
		if err != nil {
			return nil, err
		}
		s.LastMessage = lastMsg

		out = append(out, s)
	}

	return out, nil
}

func scanMessages(rows *sql.Rows) ([]Message, error) {
	var out []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(
			&m.MessageID,
			&m.SessionID,
			&m.SessionSeq,
			&m.SenderID,
			&m.Content,
			&m.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan message row: %w", err)
		}
		out = append(out, m)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate message rows: %w", err)
	}

	return out, nil
}

func reverseMessages(items []Message) {
	for i, j := 0, len(items)-1; i < j; i, j = i+1, j-1 {
		items[i], items[j] = items[j], items[i]
	}
}

func normalizeIDs(ids []string) []string {
	seen := make(map[string]struct{}, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
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

func placeholders(n int) string {
	if n <= 0 {
		return ""
	}
	parts := make([]string, n)
	for i := 0; i < n; i++ {
		parts[i] = "?"
	}
	return strings.Join(parts, ",")
}
