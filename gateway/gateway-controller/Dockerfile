# Stage 1: Build the Go application
FROM golang:1.25.1 AS builder

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . ./

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o controller cmd/controller/main.go

# Stage 2: Create minimal runtime image
FROM alpine:3.22.2

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates wget

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/controller .

# Copy default configuration file
COPY config/config.yaml /etc/gateway-controller/config.yaml

# Create data directory for bbolt database
RUN mkdir -p /data

# Expose ports
EXPOSE 9090 18000

# Set default environment variables (can override config file)
ENV GC_LOGGING_LEVEL=info
ENV GC_STORAGE_DATABASE_PATH=/data/gateway-controller.db

# Run the application
CMD ["/app/controller"]
