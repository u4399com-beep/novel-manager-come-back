# 归来小说CMS - Minimal Production Docker Image
# Build:  docker build -t novel-come-back .
# Run:    docker run -p 8008:8008 -e DATABASE_URL=... novel-come-back

FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /novel-server ./cmd/server/

# Runtime stage
FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /novel-server /novel-server
COPY web/ /web/
COPY i18n/ /i18n/

EXPOSE 8008
ENV SERVER_PORT=8008
ENV STATIC_DIR=/web/static

ENTRYPOINT ["/novel-server"]
