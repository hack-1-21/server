package db

import (
	"database/sql"
	"log"
	"os"

	_ "github.com/lib/pq"
)

var DB *sql.DB

// Init はPostgreSQLに接続し、テーブルをマイグレーションする
func Init() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		// フォールバック（ローカル開発用）
		dsn = "host=localhost port=5432 user=soundreal password=soundreal dbname=soundreal sslmode=disable"
	}

	var err error
	DB, err = sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("DB接続失敗: %v", err)
	}

	if err = DB.Ping(); err != nil {
		log.Fatalf("DBに接続できません: %v", err)
	}

	migrate()
	log.Println("DB初期化完了")
}

func migrate() {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS users (
			user_id       TEXT PRIMARY KEY,
			email         TEXT NOT NULL DEFAULT '',
			password_hash TEXT NOT NULL DEFAULT '',
			nickname      TEXT NOT NULL DEFAULT '',
			level         INTEGER NOT NULL DEFAULT 1,
			exp           INTEGER NOT NULL DEFAULT 0,
			total_exp     INTEGER NOT NULL DEFAULT 0,
			points        INTEGER NOT NULL DEFAULT 0,
			alert_enabled BOOLEAN NOT NULL DEFAULT true,
			theme         TEXT NOT NULL DEFAULT 'light',
			created_at    TIMESTAMPTZ DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS measurements (
			id         SERIAL PRIMARY KEY,
			user_id    TEXT    NOT NULL DEFAULT '',
			db         REAL    NOT NULL,
			hz         REAL    NOT NULL,
			latitude   DOUBLE PRECISION NOT NULL,
			longitude  DOUBLE PRECISION NOT NULL,
			created_at TIMESTAMPTZ DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS gardens (
			id           SERIAL PRIMARY KEY,
			user_id      TEXT NOT NULL,
			generation   INTEGER NOT NULL DEFAULT 1,
			points       INTEGER NOT NULL DEFAULT 0,
			stage        INTEGER NOT NULL DEFAULT 1,
			image_url    TEXT DEFAULT '',
			is_active    BOOLEAN NOT NULL DEFAULT TRUE,
			completed_at TIMESTAMPTZ,
			created_at   TIMESTAMPTZ DEFAULT NOW()
		)`,
		// インデックス
		`CREATE INDEX IF NOT EXISTS idx_measurements_location
			ON measurements (latitude, longitude)`,
		`CREATE INDEX IF NOT EXISTS idx_gardens_user_id
			ON gardens (user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_gardens_active
			ON gardens (user_id, is_active)`,
		// 既存テーブルへのカラム追加（安全なアップデート用）
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS password_hash TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS email TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS total_exp INTEGER NOT NULL DEFAULT 0`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email_lower
			ON users (LOWER(email))
			WHERE email <> ''`,
	}

	for _, q := range queries {
		if _, err := DB.Exec(q); err != nil {
			log.Fatalf("マイグレーション失敗: %v\nquery: %s", err, q)
		}
	}
}
