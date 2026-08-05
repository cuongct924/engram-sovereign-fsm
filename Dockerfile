# Multi-stage build for the Engram Sovereign FSM node
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git make

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the engramd binary
RUN CGO_ENABLED=0 GOOS=linux go build -o engramd ./cmd/engramd

# Runtime stage
FROM alpine:latest

WORKDIR /root

# Install runtime dependencies
RUN apk add --no-cache ca-certificates bash curl jq

# Copy binary from builder
COPY --from=builder /app/engramd /usr/local/bin/engramd

# Create node home directory (matches cmd/engramd's defaultHome())
RUN mkdir -p .engramd

# Expose ports
EXPOSE 26656 26657 26660 1317

ENTRYPOINT ["engramd"]
