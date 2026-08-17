# Dockerfile for WhatsApp-Go
# Multi-stage build for minimal production image

# Build stage
FROM golang:1.25-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make gcc musl-dev

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w -X main.version={{.Version}} -X main.commit={{.Commit}} -X main.date={{.Date}} -X main.builtBy=docker" \
    -o whatsapp-go ./cmd/whatsapp-go

# Final stage - minimal runtime image
FROM alpine:3.20

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 -S whatsapp && \
    adduser -u 1000 -S whatsapp -G whatsapp

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/whatsapp-go .

# Copy config files (if any)
# COPY --from=builder /app/configs ./configs

# Change ownership to non-root user
RUN chown -R whatsapp:whatsapp /app

# Switch to non-root user
USER whatsapp

# Expose port (if needed for HTTP API)
# EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD ./whatsapp-go version || exit 1

# Entry point
ENTRYPOINT ["./whatsapp-go"]

# Default command
CMD ["version"]