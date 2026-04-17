package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "github.com/go-sql-driver/mysql"
)

func main() {
	// Database configuration
	dbHost := getEnv("DB_HOST", "chatroom-dev-mysql.cjias2iok297.us-west-2.rds.amazonaws.com")
	dbPort := getEnv("DB_PORT", "3306")
	dbUser := getEnv("DB_USER", "admin")
	dbPassword := getEnv("DB_PASSWORD", "")
	dbName := getEnv("DB_NAME", "chatroom")

	if dbPassword == "" {
		log.Fatal("DB_PASSWORD environment variable is required")
	}

	// Connect to database
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&multiStatements=true",
		dbUser, dbPassword, dbHost, dbPort, dbName)

	log.Printf("Connecting to database: %s@%s:%s/%s", dbUser, dbHost, dbPort, dbName)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	log.Println("✅ Connected to database successfully")

	// Run migrations
	migrations := []string{
		// Users table
		`CREATE TABLE IF NOT EXISTS users (
			id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
			user_id VARCHAR(64) NOT NULL,
			nickname VARCHAR(128) NOT NULL,
			avatar_url VARCHAR(255) NULL,
			created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
			updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
			PRIMARY KEY (id),
			UNIQUE KEY uq_users_user_id (user_id),
			KEY idx_users_nickname (nickname)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,

		// Relations table
		`CREATE TABLE IF NOT EXISTS relations (
			id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
			user_id VARCHAR(64) NOT NULL,
			friend_id VARCHAR(64) NOT NULL,
			created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
			PRIMARY KEY (id),
			UNIQUE KEY uq_relations_pair (user_id, friend_id),
			KEY idx_relations_user_id (user_id),
			KEY idx_relations_friend_id (friend_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,

		// Chat sessions table
		`CREATE TABLE IF NOT EXISTS chat_sessions (
			id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
			session_id VARCHAR(64) NOT NULL,
			name VARCHAR(128) NOT NULL,
			type ENUM('direct', 'group') NOT NULL,
			creator_id VARCHAR(64) NOT NULL,
			last_seq BIGINT UNSIGNED NOT NULL DEFAULT 0,
			created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
			updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
			last_message_at DATETIME(3) NULL,
			PRIMARY KEY (id),
			UNIQUE KEY uq_chat_sessions_session_id (session_id),
			KEY idx_chat_sessions_creator_id (creator_id),
			KEY idx_chat_sessions_last_message_at (last_message_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,

		// Chat session members table
		`CREATE TABLE IF NOT EXISTS chat_session_members (
			id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
			session_id VARCHAR(64) NOT NULL,
			user_id VARCHAR(64) NOT NULL,
			role ENUM('owner', 'member') NOT NULL DEFAULT 'member',
			joined_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
			last_read_seq BIGINT UNSIGNED NOT NULL DEFAULT 0,
			PRIMARY KEY (id),
			UNIQUE KEY uq_chat_session_members_pair (session_id, user_id),
			KEY idx_chat_session_members_session_id (session_id),
			KEY idx_chat_session_members_user_id (user_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,

		// Messages table
		`CREATE TABLE IF NOT EXISTS messages (
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
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
	}

	log.Printf("Running %d migrations...\n", len(migrations))

	for i, migration := range migrations {
		log.Printf("▶️  Running migration %d/%d", i+1, len(migrations))
		if _, err := db.Exec(migration); err != nil {
			log.Fatalf("❌ Migration %d failed: %v", i+1, err)
		}
		log.Printf("✅ Migration %d/%d completed", i+1, len(migrations))
	}

	log.Println("========================================")
	log.Println("✅ All migrations completed successfully!")
	log.Println("========================================")
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
