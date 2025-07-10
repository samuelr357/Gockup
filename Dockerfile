FROM golang:1.21-alpine AS builder

# Install MySQL client
RUN apk add --no-cache mysql-client

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN go build -o mysql-backup-system main.go

FROM alpine:latest

# Install MySQL client and ca-certificates
RUN apk add --no-cache mysql-client ca-certificates

WORKDIR /app

# Copy the binary from builder
COPY --from=builder /app/mysql-backup-system .

# Create directories
RUN mkdir -p /app/backups /app/logs

# Expose port
EXPOSE 8030

# Run the application
CMD ["./mysql-backup-system"]
