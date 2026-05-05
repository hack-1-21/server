# SoundReal Server

街の環境音（dB）と位置情報を収集・共有する感覚同期型SNSマップのバックエンドAPI。

## 技術スタック

| 要素 | 選択 |
|------|------|
| 言語 | Go 1.22 |
| フレームワーク | gorilla/mux |
| DB | PostgreSQL 16 |
| 画像生成 | Gemini API (Imagen 3) |
| ストレージ | ローカルストレージ (Railway Volume) |
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
│   ├── measurement.go   # 測定データ CRUD・箱庭連動ロジック
│   ├── garden.go        # 箱庭情報・図鑑取得
│   ├── image.go         # Gemini画像生成・保存ヘルパー
│   ├── area.go          # エリア検索（ヒートマップ用）
│   ├── checkin.go       # Sound Check-In
│   └── spot.go          # エリアマスター
├── models/
│   ├── measurement.go   # 測定データ・APIリクエスト構造体
│   └── garden.go        # 箱庭・プロフィールの構造体
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

| 変数名 | 説明 |
|--------|------|
| `DATABASE_URL` | PostgreSQL 接続文字列 |
| `JWT_SECRET` | JWT署名用シークレット |
| `GEMINI_API_KEY` | Gemini APIキー（Imagen画像生成用） |
| `STORAGE_DIR` | 画像保存先ディレクトリ（デフォルト: `./data/images`） |

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
| `GET` | `/measurements/bbox` | マップ用：表示範囲内の測定データ取得 |
| `GET` | `/users/{user_id}/garden` | 現在のアクティブ箱庭取得 |
| `GET` | `/users/{user_id}/garden/history` | 図鑑：過去世代の箱庭一覧取得 |
| `GET` | `/users/{user_id}/profile` | ユーザーLv・EXP・現在の箱庭をまとめて取得 |

詳細は `openapi.yaml` を参照。

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

## 測定データとマップの取得について

### 1. 音データの投稿と箱庭の成長
WearOSデバイスから音データを送信する際、**ヘッダーに `Authorization: Bearer <device_token>` を含める**ことで、そのデバイスを紐付けたユーザー（スマホ側でログインしたユーザー）としてデータが保存されます。

送信すると、以下の処理が自動で行われます。
1. **探索ポイント加算**: 1送信 = +1pt
2. **箱庭の段階アップ**: 
   - 0〜399pt: 第1段階 (Stage 1)
   - 400〜799pt: 第2段階 (Stage 2)
   - 800〜999pt: 第3段階 (Stage 3)
   ※ ポイントが閾値を超えて段階が上がると、Gemini APIで新しい箱庭画像が生成されます（非同期）。
3. **世代交代**: 1000ptに達すると現在の箱庭が保存され、新世代 (Stage 1) がスタートします。前の世代は「図鑑」に入ります。
4. **経験値・レベルアップ**: 段階アップや世代交代時にユーザーレベルが上がります。

#### 画像生成（プロンプト・パラメータ）の変更方法
画像生成のプロンプトやGemini APIのパラメータを変更したい場合は、以下のファイルを編集してください。
- **プロンプト**: `server/handlers/image.go` の `buildPrompt()` 関数
- **パラメータ**: 同ファイルの `generateGardenImage()` 内の `imagenParameters`（アスペクト比など）

#### 生成された画像の確認方法
生成された画像は Railway の Volume に保存されます。APIのレスポンスに含まれる `image_url`（例: `/images/gardens/user-001/user-001_gen1.png`）を本番URLの後ろにくっつけてブラウザで開くと画像が見られます。
例: `https://server-production-5adf.up.railway.app/images/gardens/user-001/user-001_gen1.png`

#### 図鑑（過去の箱庭）の取得
世代交代を終えた過去の箱庭のリストは、以下のAPIで取得できます。現在育成中のアクティブな箱庭は除外され、過去の世代（第1世代、第2世代...）の最終形態がすべて配列で返ってきます。

```bash
# [Production]
curl -X GET https://server-production-5adf.up.railway.app/users/user-001/garden/history
```

**レスポンス例（JSON）:**
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
  },
  {
    "id": 2,
    "user_id": "user-001",
    "generation": 2,
    "points": 1000,
    "stage": 3,
    "image_url": "/images/gardens/user-001/user-001_gen2.png",
    "is_active": false,
    "completed_at": "2026-05-15T10:00:00Z",
    "created_at": "2026-05-01T10:00:00Z"
  }
]
```

#### 生成される画像のファイル名ルール
フロントエンドからURLを予測しやすくするため、画像ファイル名は以下の形式で保存されます。
`{user_id}_gen{generation}.png`（例: `user-001_gen1.png`）

**上書きの仕様:**
- 同じ世代（Generation）の中で段階（Stage 1 → Stage 2 → Stage 3）が上がるたびに、**同じファイル名で上書き保存**されます。これによりサーバーの容量を節約し、常にその世代の最新の画像が取得できます。
- 1000pt に達して**世代交代（Generation Up）が発生すると、ファイル名の `gen` 番号が変わる**（例: `_gen2.png` になる）ため、過去の世代の画像は上書きされずに図鑑用として残ります。



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

## テスト・デバッグ用ツール

本番環境で「ユーザー登録 → 箱庭の成長 → 画像生成 → 状態確認」の一連のフローを手動でテストするための手順です。
以下のコマンドを **Git Bash** 等で順番に実行してください。

### ステップ1: 新しいテストユーザーを作成する
まずはテスト用のユーザーを作成します（例として `test-user-99` を作成します）。
```bash
curl -X POST https://server-production-5adf.up.railway.app/auth/register \
  -H "Content-Type: application/json" \
  -d '{"user_id": "test-user-99", "email": "test99@example.com", "nickname": "Tester", "password": "password123"}'
```

### ステップ2: 箱庭にポイントを追加して強制進化させる
手動で何百回もデータを送らなくても、以下のデバッグAPIを叩くことで強制的にポイントを追加し、箱庭の進化や画像生成を引き起こせます。
```bash
# test-user-99 に 400ポイント を付与して「段階アップ（Stage 2へ）」を発生させる
curl -X POST https://server-production-5adf.up.railway.app/debug/garden/add-points \
  -H "Content-Type: application/json" \
  -d '{"user_id": "test-user-99", "points": 400}'
```
※ コマンド実行後、裏でGemini APIによる画像生成が走るため、数秒待ちます。

### ステップ3: 生成されたアクティブな画像を確認する
ブラウザで以下のURLを開き、画像が生成されているか確認します。
👉 `https://server-production-5adf.up.railway.app/images/gardens/test-user-99/test-user-99.png`

### ステップ4: 全ユーザーの箱庭状態を一覧で確認する
本番環境にどんなデータが入っているか、誰がどの箱庭ステージにいるかを一覧で確認するスクリプトです。画像が生成されている場合は、そのままブラウザで開けるリンクが表示されます。
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

### 【任意】データベースの全リセット (初期化)
ハッカソンのデモ前や、古いダミーデータでおかしくなった場合に、**データベースのすべてのユーザーと測定データを完全に消去**するAPIです。
```bash
# [Production]
curl -X DELETE https://server-production-5adf.up.railway.app/debug/reset
```
※ 実行すると `users`, `measurements`, `gardens` 等のデータが完全に空になります。

---

## API 仕様書の閲覧

`openapi.yaml` を [Swagger Editor](https://editor.swagger.io/) または VS Code の OpenAPI 拡張で開くと、
インタラクティブに API を確認できる。
