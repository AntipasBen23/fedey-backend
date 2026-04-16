# ── Stage 1: Build ────────────────────────────────────────────────────────────
FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o server .

# ── Stage 2: Runtime ──────────────────────────────────────────────────────────
FROM alpine:latest

# FFmpeg for video assembly; ttf-dejavu for slide text rendering; ca-certificates for HTTPS calls
RUN apk add --no-cache ffmpeg ttf-dejavu ca-certificates

WORKDIR /app
COPY --from=builder /app/server .

EXPOSE 8080
CMD ["./server"]
