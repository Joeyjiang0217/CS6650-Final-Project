CREATE TABLE IF NOT EXISTS messages (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  message_id VARCHAR(64) NOT NULL,
  session_id VARCHAR(64) NOT NULL,
  session_seq BIGINT UNSIGNED NOT NULL,
  sender_id VARCHAR(64) NOT NULL,
  content TEXT NOT NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uq_messages_message_id (message_id),
  UNIQUE KEY uq_messages_session_seq (session_id, session_seq),
  KEY idx_messages_session_created_at (session_id, created_at),
  KEY idx_messages_session_seq (session_id, session_seq)
) ENGINE=InnoDB
  DEFAULT CHARSET=utf8mb4
  COLLATE=utf8mb4_unicode_ci;