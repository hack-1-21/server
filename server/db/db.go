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
			user_id    TEXT PRIMARY KEY,
			nickname   TEXT NOT NULL DEFAULT '',
			level      INTEGER NOT NULL DEFAULT 1,
			exp        INTEGER NOT NULL DEFAULT 0,
			points     INTEGER NOT NULL DEFAULT 0,
			alert_enabled BOOLEAN NOT NULL DEFAULT true,
			theme      TEXT NOT NULL DEFAULT 'light',
			created_at TIMESTAMPTZ DEFAULT NOW()
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
		// ヒートマップ用: 位置情報インデックス
		`CREATE INDEX IF NOT EXISTS idx_measurements_location
			ON measurements (latitude, longitude)`,

		`CREATE TABLE IF NOT EXISTS checkins (
			id         SERIAL PRIMARY KEY,
			user_id    TEXT    NOT NULL,
			latitude   DOUBLE PRECISION NOT NULL,
			longitude  DOUBLE PRECISION NOT NULL,
			db         REAL    NOT NULL,
			message    TEXT    DEFAULT '',
			created_at TIMESTAMPTZ DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_checkins_user
			ON checkins (user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_checkins_location
			ON checkins (latitude, longitude)`,
	}

	for _, q := range queries {
		if _, err := DB.Exec(q); err != nil {
			log.Fatalf("マイグレーション失敗: %v\nquery: %s", err, q)
		}
	}
}
