# ---- 构建阶段 ----
FROM golang:1.21-alpine AS builder

WORKDIR /src
COPY go.mod ./
RUN go mod download || true
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/app ./cmd/server

# ---- 运行阶段 ----
FROM alpine:latest
WORKDIR /app
RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /out/app ./app
COPY config.example.json ./config.example.json

ENV CONFIG_FILE=/app/config.json
ENV PORT=5000

EXPOSE 5000
CMD ["./app"]
