# Multi-stage build for the mock OTLP collector
FROM golang:1.26.5-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY *.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -o mock-otlp-collector .

FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

COPY --from=builder /app/mock-otlp-collector .

EXPOSE 4317 4318 8080

CMD ["./mock-otlp-collector"]
