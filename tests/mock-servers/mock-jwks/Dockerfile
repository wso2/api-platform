FROM golang:1.23-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o mock-jwks main.go

FROM alpine:latest

WORKDIR /app
COPY --from=builder /app/mock-jwks .

EXPOSE 8080
CMD ["./mock-jwks"]
