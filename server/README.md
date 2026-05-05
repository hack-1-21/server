# SoundReal Server

街の環境音（dB）と位置情報を収集・共有する感覚同期型SNSマップのバックエンドAPI。
ユーザーが音データを収集するたびに「探索ポイント」が貯まり、瓶の中に育つ幻想的な箱庭（AI生成画像）が成長していく。

## 技術スタック

| 要素 | 選択 |
|------|------|
| 言語 | Go 1.22 |
| フレームワーク | gorilla/mux |
| DB | PostgreSQL 16 |
| 画像生成 | Cloudflare Workers AI (`@cf/stabilityai/stable-diffusion-xl-base-1.0`) |
| 画像ストレージ | Railway Volume（ローカルファイルシステム） |
| コンテナ | Docker / Docker Compose |

## ディレクトリ構成

```
server/
├── main.go              # エントリポイント・ルーター
├── Dockerfile
├── docker-compose.yml
├── go.mod
├── db/
│   └── db.go            # PostgreSQL 接続・マイグレーション
├── handlers/
│   ├── helpers.go       # 共通レスポンスヘルパー
│   ├── auth.go          # 認証（登録・ログイン・Google）
│   ├── measurement.go   # 音データ投稿・箱庭連動ロジック
│   ├── garden.go        # 箱庭情報・図鑑取得
│   ├── image.go         # Cloudflare AI 画像生成・ローカル保存
│   ├── debug.go         # テスト・デバッグ用エンドポイント
│   └── area.go          # エリア検索（バウンディングボックス）
├── models/
│   ├── user.go          # ユーザー構造体
│   ├── measurement.go   # 測定データ構造体
│   └── garden.go        # 箱庭・プロフィール構造体
```

---

## 環境変数

### Railway（本番）に設定が必要な変数

| 変数名 | 説明 |
|--------|------|
| `DATABASE_URL` | PostgreSQL 接続文字列（Railwayが自動生成） |
| `JWT_SECRET` | JWT署名用シークレット（任意の文字列） |
| `CF_ACCOUNT_ID` | Cloudflare アカウントID |
| `CF_API_TOKEN` | Cloudflare Workers AI 権限付きAPIトークン |
| `STORAGE_DIR` | 画像保存先ディレクトリ（Railway Volumeにマウント: `/app/data/images`） |

> **Railway Volume の設定**: Railway ダッシュボード → Volumes → New Volume → マウントパス `/app/data` に設定する。

### ローカル開発用（docker-compose.yml に記載）

| 変数名 | 説明 |
|--------|------|
| `DATABASE_URL` | `postgresql://postgres:password@db:5432/soundreal?sslmode=disable` |
| `JWT_SECRET` | `dev-secret` 等でOK |
| `CF_ACCOUNT_ID` | Cloudflare アカウントID（ローカルでも画像生成をテストする場合） |
| `CF_API_TOKEN` | Cloudflare Workers AI 権限付きAPIトークン |
| `STORAGE_DIR` | `./data/images`（未設定時のデフォルト値） |

---

## ローカル開発環境セットアップ

### 前提条件

- Docker & Docker Compose がインストール済みであること

### 起動

```bash
cd server

# 依存関係の解決（初回のみ）
# Linux / macOS
docker run --rm -v "$(pwd):/app" -w /app golang:1.22-alpine go mod tidy

# Windows (PowerShell)
docker run --rm -v "${PWD}:/app" -w /app golang:1.22-alpine go mod tidy

# サーバー起動（PostgreSQL + API サーバー）
docker compose up --build
```

---

## 本番環境（Railway）

本プロジェクトは [Railway](https://railway.app) にデプロイされています。
`main` ブランチへの `git push` で自動的に再デプロイされます。

### 本番URL

```
https://server-production-5adf.up.railway.app
```

### デプロイ設定

- **Root Directory**: `server`（Railwayの Settings → Root Directory に設定）
- **マイグレーション**: サーバー起動時に `db.Init()` が自動でテーブルを作成するため、手動操作は不要。

---

## API エンドポイント

### 認証

| Method | Path | 説明 |
|--------|------|------|
| `POST` | `/auth/register` | 新規ユーザー登録。成功時に JWTトークンを返す。**登録と同時に箱庭の初期画像（Stage 1）を自動生成する。** |
| `POST` | `/auth/login` | メール・パスワードでログイン |
| `POST` | `/auth/google` | Google `id_token` でログイン/新規登録 |

### ユーザー

| Method | Path | 説明 |
|--------|------|------|
| `GET` | `/users/{user_id}` | ユーザープロフィール取得 |
| `PUT` | `/users/{user_id}` | ニックネーム・テーマ等の更新 |
| `GET` | `/users/{user_id}/profile` | ユーザーLv・EXP・現在の箱庭をまとめて取得 |

### 箱庭（Garden）

| Method | Path | 説明 |
|--------|------|------|
| `GET` | `/users/{user_id}/garden` | 現在育成中のアクティブ箱庭を取得 |
| `GET` | `/users/{user_id}/garden/history` | 図鑑：世代交代した過去の箱庭を一覧で取得 |

### 音データ（Measurement）

| Method | Path | 説明 |
|--------|------|------|
| `POST` | `/measurements` | WearOS から音データ投稿（箱庭ポイント加算・進化トリガー） |
| `GET` | `/measurements` | 全件または差分取得（`?after_id=N`） |
| `GET` | `/measurements/bbox` | マップ表示範囲内のデータ取得 |

### デバッグ（開発・テスト用）

| Method | Path | 説明 |
|--------|------|------|
| `GET` | `/health` | ヘルスチェック |
| `GET` | `/debug/users` | 登録済みの全ユーザーID一覧 |
| `POST` | `/debug/garden/add-points` | 指定ユーザーに強制的にポイントを追加し、進化をシミュレート |
| `DELETE` | `/debug/reset` | ⚠️ DBの全データ削除（users / measurements / gardens） |
| `GET` | `/debug/check-cf-config` | Cloudflare 環境変数の設定確認 |
| `GET` | `/debug/test-cloudflare` | Cloudflare Workers AI に実際に接続して画像生成をテスト |

---

## 認証とゲスト対応について

### 1. ユーザー登録とログイン
このAPIはJWT (JSON Web Token) を用いた認証の基礎を提供しています。

**【推奨】Googleログイン (フロントエンド連携)**
- **Googleログイン (`/auth/google`)**:
  フロントエンド(React Native/Expo等)で取得したGoogleの `id_token` をサーバーに送信すると、サーバー側で検証し、独自JWT `token` とユーザー情報を返却します。
  ※これが最も簡単で安全な実装方法です。

**【任意】メールアドレス/パスワード形式**
- **新規登録 (`/auth/register`)**: `email`, `nickname`, `password` を送信して作成。`user_id` は省略可能で、省略時はサーバー側で生成されます。
- **ログイン (`/auth/login`)**: `email`, `password` で検証。

（※ ハッカソン用途のため、現状では各エンドポイントでのJWTの厳密な検証ミドルウェアは省略していますが、フロントエンド側で取得した `token` を保持しておいてください。）

#### 動作確認 (QAテスト): ユーザー登録とログイン
環境に合わせて、ローカルか本番(Production)のURLを選択して実行してください。
※ Windowsの場合は、コマンドプロンプトやPowerShellではなく **Git Bash** を使用するとそのままコピペで実行できます。

```bash
# ----- 新規登録 -----
# [Local]
curl -X POST http://localhost:8080/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email": "test@example.com", "nickname": "TestUser", "password": "password123"}'

# [Production]
curl -X POST https://server-production-5adf.up.railway.app/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email": "test@example.com", "nickname": "TestUser", "password": "password123"}'


# ----- ログイン (JWT取得) -----
# [Local]
curl -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email": "test@example.com", "password": "password123"}'

# [Production]
curl -X POST https://server-production-5adf.up.railway.app/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email": "test@example.com", "password": "password123"}'


# ----- Google ログイン -----
# フロントエンドから取得した実際の id_token を入れてテストしてください
# [Local]
curl -X POST http://localhost:8080/auth/google \
  -H "Content-Type: application/json" \
  -d '{"id_token": "YOUR_GOOGLE_ID_TOKEN_HERE"}'

# [Production]
curl -X POST https://server-production-5adf.up.railway.app/auth/google \
  -H "Content-Type: application/json" \
  -d '{"id_token": "YOUR_GOOGLE_ID_TOKEN_HERE"}'
```

### 2. ゲストユーザーの扱い
ハッカソンのデモ等で一時的な「ゲスト」としてアプリに入る場合、**バックエンド側に専用のアカウントを作成する必要はありません**。
- `/measurements/bbox` などのデータ取得APIは誰でも（トークン無しでも）アクセス可能です。
- ゲストは `user_id` を持たないため、「他人が取った音のマップ」は閲覧できますが、「自分の取った音のマップ」の取得条件 (`&user_id=...` の指定) は利用できない、という形で自然にアクセス制限が実現されます。

---

## 箱庭システムの仕様

### ゲームループ

1. WearOS が `POST /measurements` にデータを送るたびに **+1ポイント** が加算される
2. ポイントに応じて自動的に段階（Stage）が上がり、**Cloudflare Workers AI で新しい箱庭画像が非同期生成される**
3. 1000pt に達すると**世代交代（Generation Up）**し、前の世代は「図鑑」に保存される

### 段階（Stage）の定義

| ポイント範囲 | Stage | 画像の世界観 |
|---|---|---|
| 0 〜 399 pt | Stage 1 | 小さな芽吹き・静かな霧の中 |
| 400 〜 799 pt | Stage 2 | 成長した木・虹・温かな光 |
| 800 〜 999 pt | Stage 3 | 古代の巨木・輝く妖精・満ち溢れる生命力 |

### 画像生成プロンプトの変更方法

`handlers/image.go` の `buildPrompt()` 関数を直接編集してください。
ステージごとに個別のプロンプトが定義されています。

#### 世代ごとのテーマ固定（季節・天候）

**同一世代内（Stage 1→2→3）では、季節と天候が必ず同じ**になるよう実装されています。
`generation` 番号を乱数シードとして使うことで、同じ世代のすべての画像に統一されたテーマが適用されます。

| 要素 | 固定方法 | 例 |
|---|---|---|
| 季節 (`season`) | 世代番号でシード固定 | 第1世代は必ず「autumn」など |
| 天候 (`weather`) | 世代番号でシード固定 | 第1世代は必ず「misty」など |
| 動物 (`animal`) | 毎回ランダム | Stage 1は兎、Stage 2は鹿…とバラバラでOK |

世代が変わると（Generation 2, 3, …）、異なる季節・天候の組み合わせが選ばれるため、各世代が自然に独自のテーマを持ちます。

---

## 画像ファイルのURL仕様

> **⚠️ Cloudflare Workers AI（無料プラン）の制限に注意**
> 画像生成モデル（`stable-diffusion-xl-base-1.0`）は **1日あたりの生成枚数に上限**があります。
> 無料プランの場合、生成枚数がリセットされるのは**毎日午前9時（JST）**頃です。
> ハッカソン当日・デモ前は**過度な連続生成を避け**、テストは計画的に行ってください。

### アクティブな箱庭の画像（常に最新）

フロントエンドが現在育成中の箱庭画像を取得する場合、**世代番号を意識せずに常に以下の固定URLで取得できます。**

```
https://server-production-5adf.up.railway.app/images/gardens/{user_id}/{user_id}.png
```

例: `https://server-production-5adf.up.railway.app/images/gardens/user-001/user-001.png`

- 段階が上がるたびに**同一URLで上書き保存**されるため、常に最新の形態が表示される
- フロントはこのURLをキャッシュバスト付きで表示するだけでOK

### 図鑑（過去の世代）の画像

世代交代が完了した箱庭の画像は、以下のURL形式で**永続的に保存**されます。

```
https://server-production-5adf.up.railway.app/images/gardens/{user_id}/{user_id}_gen{N}.png
```

例:
- 第1世代の最終形態: `.../user-001/user-001_gen1.png`
- 第2世代の最終形態: `.../user-001/user-001_gen2.png`

図鑑API（`GET /users/{user_id}/garden/history`）のレスポンスには `image_url` フィールドが含まれており、フロントはそのURLをそのままブラウザで表示できます。

### 図鑑APIのレスポンス例

```json
[
  {
    "id": 1,
    "user_id": "user-001",
    "generation": 1,
    "points": 1000,
    "stage": 3,
    "image_url": "/images/gardens/user-001/user-001_gen1.png",
    "is_active": false,
    "completed_at": "2026-05-01T10:00:00Z",
    "created_at": "2026-04-30T10:00:00Z"
  }
]
```

フル画像URLは `https://server-production-5adf.up.railway.app` + `image_url` で構築してください。

---

## テスト・デバッグ手順

本番環境で「ユーザー登録 → 箱庭の成長 → 画像確認 → 世代交代」の一連のフローをテストする手順です。
以下のコマンドを **Git Bash** で順番に実行してください。

### ステップ0: Cloudflare 接続確認

```bash
curl https://server-production-5adf.up.railway.app/debug/check-cf-config
# => {"CF_ACCOUNT_ID_set":true,"CF_API_TOKEN_set":true,"configured":true} であればOK
```

### ステップ1: テストユーザーを新規作成

```bash
curl -X POST https://server-production-5adf.up.railway.app/auth/register \
  -H "Content-Type: application/json" \
  -d '{"user_id": "test-user-01", "email": "test01@example.com", "nickname": "Tester", "password": "password123"}'
```

登録と同時に Stage 1 の初期画像が**非同期で生成**されます（数秒〜15秒程度かかります）。

### ステップ2: 箱庭の状態を確認

```bash
curl -s https://server-production-5adf.up.railway.app/users/test-user-01/garden
```

`image_url` に値が入っていれば画像生成が完了しています。
ブラウザで直接確認:
👉 `https://server-production-5adf.up.railway.app/images/gardens/test-user-01/test-user-01.png`

### ステップ3: ポイントを追加して段階アップをシミュレート

```bash
# Stage 2へ進化（400pt追加）
curl -X POST https://server-production-5adf.up.railway.app/debug/garden/add-points \
  -H "Content-Type: application/json" \
  -d '{"user_id": "test-user-01", "points": 400}'

# Stage 3へ進化（さらに400pt追加）
curl -X POST https://server-production-5adf.up.railway.app/debug/garden/add-points \
  -H "Content-Type: application/json" \
  -d '{"user_id": "test-user-01", "points": 400}'

# 世代交代（さらに200pt追加 → 合計1000pt超）
curl -X POST https://server-production-5adf.up.railway.app/debug/garden/add-points \
  -H "Content-Type: application/json" \
  -d '{"user_id": "test-user-01", "points": 200}'
```

### ステップ4: 全ユーザーの箱庭状態を一覧確認

```bash
users=$(curl -s https://server-production-5adf.up.railway.app/debug/users | grep -o '"[^"]*"' | tr -d '"')
if [ -z "$users" ] || [ "$users" == "null" ]; then
  echo "データが空です"
else
  echo "--- 登録されている箱庭一覧 ---"
  for user in $users; do
    echo "UserID: $user"
    res=$(curl -s "https://server-production-5adf.up.railway.app/users/$user/garden")
    echo "  Data: $res"
    img=$(echo $res | grep -o '"image_url":"[^"]*' | cut -d'"' -f4)
    if [ -n "$img" ]; then
      echo "  Image Link: https://server-production-5adf.up.railway.app$img"
    fi
    echo ""
  done
fi
```

### 【任意】DBの全リセット

デモ前やデータが壊れた場合に使用。**実行すると全データが消えます。**

```bash
curl -X DELETE https://server-production-5adf.up.railway.app/debug/reset
```

---

## マップ用データ取得（バウンディングボックス）

```bash
# 指定範囲内の全員の測定データを取得
curl "https://server-production-5adf.up.railway.app/measurements/bbox?ne_lat=35.690&ne_lng=139.770&sw_lat=35.670&sw_lng=139.750"

# 特定ユーザーのデータのみ絞り込む
curl "https://server-production-5adf.up.railway.app/measurements/bbox?ne_lat=35.690&ne_lng=139.770&sw_lat=35.670&sw_lng=139.750&user_id=user-001"
```
---

## API 仕様書の閲覧

`openapi.yaml` を [Swagger Editor](https://editor.swagger.io/) または VS Code の OpenAPI 拡張で開くと、
インタラクティブに API を確認できる。