.PHONY: build test clean

MODULE_NAME := event-gateway
BINARY_NAME := event-gateway

build:
	go build -o bin/$(BINARY_NAME) ./cmd/event-gateway/

test:
	go test ./...

clean:
	rm -rf bin/
