# Multi-stage build for mock analytics collector
FROM golang:1.26.2-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY main.go ./

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -o mock-analytics-collector main.go

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

COPY --from=builder /app/mock-analytics-collector .

EXPOSE 8080

CMD ["./mock-analytics-collector"]
