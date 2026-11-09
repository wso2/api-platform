# Multi-stage build for the mock streaming LLM backend
FROM golang:1.26.5-alpine AS builder

WORKDIR /app

COPY go.mod ./

COPY *.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -o mock-llm-stream .

FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

COPY --from=builder /app/mock-llm-stream .

EXPOSE 8080

CMD ["./mock-llm-stream"]
