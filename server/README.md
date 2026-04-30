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

## 認証とゲスト対応について

### 1. ユーザー登録とログイン
このAPIはJWT (JSON Web Token) を用いた認証の基礎を提供しています。
- **新規登録 (`/auth/register`)**: `user_id`, `nickname`, `password` を送信すると、ユーザーが作成され JWT `token` が返却されます。
- **ログイン (`/auth/login`)**: `user_id`, `password` を送信すると、検証のうえ JWT `token` が返却されます。

（※ ハッカソン用途のため、現状では各エンドポイントでのJWTの厳密な検証ミドルウェアは省略していますが、フロントエンド側でトークンを保持してヘッダに付与するなどのフローを実装可能です。）

#### 動作確認 (QAテスト): ユーザー登録とログイン
環境に合わせて、ローカルか本番(Production)のURLを選択して実行してください。
※ Windowsの場合は、コマンドプロンプトやPowerShellではなく **Git Bash** を使用するとそのままコピペで実行できます。

```bash
# ----- 新規登録 -----
# [Local]
curl -X POST http://localhost:8080/auth/register \
  -H "Content-Type: application/json" \
  -d '{"user_id": "user-001", "nickname": "TestUser", "password": "password123"}'

# [Production]
curl -X POST https://server-production-5adf.up.railway.app/auth/register \
  -H "Content-Type: application/json" \
  -d '{"user_id": "user-001", "nickname": "TestUser", "password": "password123"}'


# ----- ログイン (JWT取得) -----
# [Local]
curl -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{"user_id": "user-001", "password": "password123"}'

# [Production]
curl -X POST https://server-production-5adf.up.railway.app/auth/login \
  -H "Content-Type: application/json" \
  -d '{"user_id": "user-001", "password": "password123"}'
```

### 2. ゲストユーザーの扱い
ハッカソンのデモ等で一時的な「ゲスト」としてアプリに入る場合、**バックエンド側に専用のアカウントを作成する必要はありません**。
- `/measurements/bbox` などのデータ取得APIは誰でも（トークン無しでも）アクセス可能です。
- ゲストは `user_id` を持たないため、「他人が取った音のマップ」は閲覧できますが、「自分の取った音のマップ」の取得条件 (`&user_id=...` の指定) は利用できない、という形で自然にアクセス制限が実現されます。

---

## 測定データとマップの取得について

### 1. 音データの投稿 (WearOSからの送信)
WearOSデバイス等から音データを送信すると、DBに保存されると同時に、ユーザーに経験値が付与されレベルアップの判定が行われます。

#### 動作確認 (QAテスト): 音データ投稿
```bash
# [Local]
curl -X POST http://localhost:8080/measurements \
  -H "Content-Type: application/json" \
  -d '{"user_id": "user-001", "db": 65.5, "hz": 440.0, "latitude": 35.6812, "longitude": 139.7671}'

# [Production]
# curl -X POST https://server-production-5adf.up.railway.app/measurements \
#   -H "Content-Type: application/json" \
#   -d '{"user_id": "user-001", "db": 65.5, "hz": 440.0, "latitude": 35.6812, "longitude": 139.7671}'
```

### 2. マップ用データ取得 (バウンディングボックス)
マップ画面の表示範囲（北東と南西の緯度経度）を指定して、そこに含まれるすべてのデータを取得します。

#### 動作確認 (QAテスト): マップ用データ取得
```bash
# ----- 全員のデータ（他者のデータ）を取得 -----
# [Local]
curl "http://localhost:8080/measurements/bbox?ne_lat=35.690&ne_lng=139.770&sw_lat=35.670&sw_lng=139.750"

# [Production]
# curl "https://server-production-5adf.up.railway.app/measurements/bbox?ne_lat=35.690&ne_lng=139.770&sw_lat=35.670&sw_lng=139.750"


# ----- 自分のデータのみを取得 -----
# [Local]
curl "http://localhost:8080/measurements/bbox?ne_lat=35.690&ne_lng=139.770&sw_lat=35.670&sw_lng=139.750&user_id=user-001"

# [Production]
# curl "https://server-production-5adf.up.railway.app/measurements/bbox?ne_lat=35.690&ne_lng=139.770&sw_lat=35.670&sw_lng=139.750&user_id=user-001"
```

---

## 開発・デバッグ用ツール

### データベースの全リセット (初期化)
ハッカソンのデモ前や、古いダミーデータでおかしくなった場合に、**データベースのすべてのユーザーと測定データを完全に消去**するAPIです。

```bash
# [Local]
curl -X DELETE http://localhost:8080/debug/reset

# [Production]
# curl -X DELETE https://server-production-5adf.up.railway.app/debug/reset
```
※ 実行すると `users` と `measurements` テーブルが空になり、最初のクリーンな状態に戻ります。

---

## API 仕様書の閲覧

`openapi.yaml` を [Swagger Editor](https://editor.swagger.io/) または VS Code の OpenAPI 拡張で開くと、
インタラクティブに API を確認できる。