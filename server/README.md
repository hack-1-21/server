# SoundReal Server

街の環境音（dB）と位置情報を収集・共有する感覚同期型SNSマップのバックエンドAPI。

## 技術スタック

| 要素 | 選択 |
|------|------|
| 言語 | Go 1.22 |
| フレームワーク | gorilla/mux |
| DB | PostgreSQL 16 |
| コンテナ | Docker / Docker Compose |
| API 仕様 | OpenAPI 3.0.3 (`openapi.yaml`) |

## ディレクトリ構成

```
server/
├── main.go              # エントリポイント・ルーター
├── openapi.yaml         # API 仕様書（OpenAPI 3.0.3）
├── Dockerfile
├── docker-compose.yml
├── go.mod
├── go.sum
├── db/
│   └── db.go            # PostgreSQL 接続・マイグレーション
├── handlers/
│   ├── helpers.go       # 共通レスポンスヘルパー
│   ├── measurement.go   # 測定データ CRUD
│   ├── area.go          # エリア検索（ヒートマップ用）
│   ├── checkin.go       # Sound Check-In
│   └── spot.go          # エリアマスター
└── models/
    └── measurement.go   # 全構造体定義
```

## セットアップ

### 前提条件

- Docker & Docker Compose がインストール済みであること

### 起動

```bash
cd server

# 依存関係の解決（初回のみ）
docker run --rm -v "$(pwd):/app" -w /app golang:1.22-alpine go mod tidy

# サーバー起動（PostgreSQL + API サーバー）
docker compose up --build
```

> **Windows (PowerShell) の場合:**
> ```powershell
> docker run --rm -v "${PWD}:/app" -w /app golang:1.22-alpine go mod tidy
> docker compose up --build
> ```

### 環境変数

| 変数名 | デフォルト | 説明 |
|--------|-----------|------|
| `DATABASE_URL` | `postgres://soundreal:soundreal@postgres:5432/soundreal?sslmode=disable` | PostgreSQL 接続文字列 |

## API エンドポイント

| Method | Path | 説明 |
|--------|------|------|
| `GET` | `/health` | ヘルスチェック |
| `POST` | `/measurements` | 音データ投稿（WearOS から） |
| `GET` | `/measurements` | 最新100件取得 |
| `GET` | `/measurements/latest` | 最新1件取得 |
| `GET` | `/measurements/area` | ヒートマップ用エリア検索 |
| `POST` | `/checkins` | Sound Check-In 投稿 |
| `GET` | `/checkins` | SNS フィード（最新50件） |
| `GET` | `/areas/{radius_km}/master` | エリアマスター取得 |

詳細は `openapi.yaml` を参照。

## 動作確認（curl）

```bash
# ヘルスチェック
curl http://localhost:8080/health

# 音データ投稿（WearOS 送信をエミュレート）
curl -X POST http://localhost:8080/measurements \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "user-001",
    "db": 65.5,
    "hz": 440.0,
    "latitude": 35.6812,
    "longitude": 139.7671
  }'

# ヒートマップ用エリア検索（渋谷周辺 1km）
curl "http://localhost:8080/measurements/area?lat=35.6812&lng=139.7671&radius=1.0"

# Sound Check-In 投稿
curl -X POST http://localhost:8080/checkins \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "user-001",
    "latitude": 35.6812,
    "longitude": 139.7671,
    "db": 72.0,
    "message": "渋谷スクランブル交差点！"
  }'

# エリアマスター取得（渋谷周辺 500m）
curl "http://localhost:8080/areas/0.5/master?lat=35.6812&lng=139.7671"
```

## API 仕様書の閲覧

`openapi.yaml` を [Swagger Editor](https://editor.swagger.io/) または VS Code の OpenAPI 拡張で開くと、
インタラクティブに API を確認できる。
