# Multi-stage Dockerfile for MCP LocalBridge

# Stage 1: Build stage
FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make

# Set working directory
WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o mcp-server ./cmd/server/main.go

# Stage 2: Runtime stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 mcp && \
    adduser -D -u 1000 -G mcp mcp

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/mcp-server .

# Copy configuration
COPY --from=builder /build/config/config.yaml ./config/

# Change ownership
RUN chown -R mcp:mcp /app

# Switch to non-root user
USER mcp

# Expose ports for HTTP and SSE transports
EXPOSE 8080 8081

# Set default command
ENTRYPOINT ["/app/mcp-server"]
CMD ["-config", "/app/config/config.yaml"]
