# Stage 1: Build the Go application
FROM golang:1.25.14-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git

# Set working directory
WORKDIR /build

# Copy go module files from project root
COPY go.mod go.sum ./
RUN go mod download

# Copy the API source code
COPY apps/api/ ./apps/api/

# Build a static binary; VERSION is reported by /health
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o server ./apps/api/cmd/server

# Stage 2: Create minimal runtime image
FROM alpine:3.24.1

# Install CA certificates for HTTPS
RUN apk --no-cache add ca-certificates

# Create a non-root user
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

WORKDIR /app

# Copy the binary from builder
COPY --from=builder /build/server .

# Copy migrations
COPY --from=builder /build/apps/api/migrations ./migrations

# Copy Docker-specific config
COPY apps/api/config.docker.yaml ./config.yaml

# Change ownership to non-root user
RUN chown -R appuser:appuser /app

# Switch to non-root user
USER appuser

# Expose port
EXPOSE 8080

# Run the binary
CMD ["./server"]
