FROM golang:1.26.5-alpine AS builder

WORKDIR /app

COPY go.mod ./

COPY . .
RUN go build -o mock-oauth2-idp main.go

FROM alpine:latest

WORKDIR /app
COPY --from=builder /app/mock-oauth2-idp .

EXPOSE 9601
CMD ["./mock-oauth2-idp"]
