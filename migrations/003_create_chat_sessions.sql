CREATE TABLE IF NOT EXISTS chat_sessions (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  session_id VARCHAR(64) NOT NULL,
  name VARCHAR(128) NOT NULL,
  type ENUM('direct', 'group') NOT NULL,
  creator_id VARCHAR(64) NOT NULL,
  last_seq BIGINT UNSIGNED NOT NULL DEFAULT 0,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
    ON UPDATE CURRENT_TIMESTAMP(3),
  last_message_at DATETIME(3) NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_chat_sessions_session_id (session_id),
  KEY idx_chat_sessions_creator_id (creator_id),
  KEY idx_chat_sessions_last_message_at (last_message_at)
) ENGINE=InnoDB
  DEFAULT CHARSET=utf8mb4
  COLLATE=utf8mb4_unicode_ci;