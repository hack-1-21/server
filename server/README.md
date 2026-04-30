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

## ローカル開発環境セットアップ

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

---

## 本番環境デプロイ（Railway）

本プロジェクトは [Railway](https://railway.app) にデプロイされています。
`main` ブランチへの `git push` で自動的に再デプロイされます。

### 本番URL

```
https://server-production-5adf.up.railway.app
```

### 初回デプロイ手順（参考）

1. [railway.app](https://railway.app) にGitHubアカウントでログイン
2. 「New Project」→「Database」→「PostgreSQL」でDB作成
3. 「Add Service」→「GitHub Repository」でこのリポジトリを選択
4. Settingsの「Root Directory」に `server` を設定（重要）
5. Variablesタブで環境変数を追加：
   ```
   DATABASE_URL = ${{Postgres.DATABASE_URL}}
   ```
6. Settings → Networking → 「Generate Domain」でURLを発行

> **マイグレーションについて:** サーバー起動時に `db.Init()` が自動でテーブルを作成するため、手動マイグレーションは不要。

---

## API エンドポイント

| Method | Path | 説明 |
|--------|------|------|
| `GET` | `/health` | ヘルスチェック |
| `POST` | `/auth/register` | 新規ユーザー登録 (JWT トークン発行) |
| `POST` | `/auth/login` | ログイン (JWT トークン発行) |
| `GET` | `/users/{user_id}` | ユーザープロフィール取得 |
| `PUT` | `/users/{user_id}` | ユーザープロフィール・設定更新 |
| `POST` | `/measurements` | 音データ投稿（WearOS から）。経験値・レベルアップ処理含む |
| `GET` | `/measurements` | 全件または差分取得 (`?after_id=`) |
| `GET` | `/measurements/bbox` | マップ用：表示範囲内の測定データ取得 (`?ne_lat=&ne_lng=&sw_lat=&sw_lng=&user_id=`) |

詳細は `openapi.yaml` を参照。

---

## 動作確認

### macOS / Linux / Git Bash (Windows)

```bash
# ヘルスチェック
curl http://localhost:8080/health

# 新規登録
curl -X POST http://localhost:8080/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "user-001",
    "nickname": "TestUser",
    "password": "password123"
  }'

# ログイン (JWT取得)
curl -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "user-001",
    "password": "password123"
  }'

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

# マップ用データ取得（渋谷周辺のバウンディングボックス）
# ne_lat/ne_lng: 北東、sw_lat/sw_lng: 南西
curl "http://localhost:8080/measurements/bbox?ne_lat=35.685&ne_lng=139.770&sw_lat=35.670&sw_lng=139.750"

# 自分のデータのみマップで表示する場合
curl "http://localhost:8080/measurements/bbox?ne_lat=35.685&ne_lng=139.770&sw_lat=35.670&sw_lng=139.750&user_id=user-001"
```

### Windows (PowerShell)

> **注意:** PowerShell では `curl` が別コマンドのエイリアスになっているため、`Invoke-RestMethod` を使うこと。
> または **Git Bash** を使えば上記の curl コマンドがそのまま動く（推奨）。

```powershell
# 新規登録
Invoke-RestMethod -Method POST -Uri "http://localhost:8080/auth/register" `
  -ContentType "application/json" `
  -Body '{"user_id":"user-001","nickname":"TestUser","password":"password123"}'

# マップ用データ取得
Invoke-RestMethod -Uri "http://localhost:8080/measurements/bbox?ne_lat=35.685&ne_lng=139.770&sw_lat=35.670&sw_lng=139.750"
```

---

## API 仕様書の閲覧

`openapi.yaml` を [Swagger Editor](https://editor.swagger.io/) または VS Code の OpenAPI 拡張で開くと、
インタラクティブに API を確認できる。