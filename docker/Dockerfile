# Multi-stage build for Go application
FROM golang:1.27.1-alpine AS builder

WORKDIR /app

# Download dependencies first (caching layer)
COPY go.mod go.sum ./
RUN go mod download

# Copy source code and build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags '-s -w -extldflags "-static"' -o /app/appbin .

# Final lean Alpine runtime image
FROM alpine:3.23
LABEL maintainer="Author <reginaldsaja98@gmail.com>"

# Install CA certificates and create unprivileged user
RUN apk update && apk add --no-cache \
    ca-certificates \
    tzdata \
    && rm -rf /var/cache/apk/* \
    && adduser -D -h /home/appuser appuser

USER appuser

# Copy assets and binary
COPY --from=builder /app/db/migrations /home/appuser/app/db/migrations
COPY --from=builder /app/appbin /home/appuser/app/appbin

WORKDIR /home/appuser/app

ENV TZ=Asia/Jakarta
# Expose ports: 8080 (HTTP), 2000 (Health), 9090 (Metrics)
EXPOSE 8080 2000 9090

CMD ["./appbin"]
