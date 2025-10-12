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
FROM alpine:latest

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates wget

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/controller .

# Create data directory for bbolt database
RUN mkdir -p /data

# Expose ports
EXPOSE 9090 18000

# Set default environment variables
ENV LOG_LEVEL=info
ENV DB_PATH=/data/gateway-controller.db
ENV API_PORT=9090
ENV XDS_PORT=18000

# Run the application
CMD ["/app/controller"]
