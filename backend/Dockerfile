FROM golang:1.23-alpine AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /app/main ./cmd/server

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/main /app/main
# Install certificates for external API calls (DeepSeek, Telegram)
RUN apk --no-cache add ca-certificates

CMD ["/app/main"]