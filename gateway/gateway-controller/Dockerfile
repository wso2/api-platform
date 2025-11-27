# Stage 1: Build the Go application
FROM golang:1.25.1-alpine AS builder

WORKDIR /build

# Install build dependencies for SQLite (CGO requirement)
RUN apk add --no-cache gcc musl-dev sqlite-dev

# Copy policy-engine SDK (needed for go.mod replace directive)
COPY policy-engine/sdk /policy-engine/sdk

# Copy go mod files
COPY gateway-controller/go.mod gateway-controller/go.sum ./
RUN go mod download

# Copy source code
COPY gateway-controller/ ./

# Build the binary with CGO enabled (required for mattn/go-sqlite3)
# Static linking to avoid runtime dependencies
RUN CGO_ENABLED=1 GOOS=linux go build -a -ldflags '-linkmode external -extldflags "-static"' -o controller cmd/controller/main.go

# Stage 2: Create minimal runtime image
FROM alpine:3.22.2

# Install runtime dependencies
RUN apk --no-cache add ca-certificates wget

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/controller .

# Copy default configuration file
COPY gateway-controller/config/config.yaml /app/config/config.yaml

# Copy policy definitions
COPY gateway-controller/policies /app/policies

# Create data directory for SQLite database
RUN mkdir -p /app/data

# Expose ports
EXPOSE 9090 18000

# Run the application
ENTRYPOINT ["/app/controller"]
