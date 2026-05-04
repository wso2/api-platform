FROM golang:1.26.2-alpine AS builder
RUN apk add --no-cache gcc musl-dev sqlite-dev
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY main.go ./
RUN CGO_ENABLED=1 GOOS=linux go build -o mock-platform-api .

FROM alpine:3.21
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /app/mock-platform-api .
EXPOSE 9243 9244
ENTRYPOINT ["./mock-platform-api"]
