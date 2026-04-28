# ビルドステージ
FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o app-server .

# 実行ステージ（軽量イメージ）
FROM alpine:3.19

WORKDIR /app

COPY --from=builder /app/app-server .

EXPOSE 8080

CMD ["./app-server"]
