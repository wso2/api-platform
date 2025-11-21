#!/bin/bash

# Script to run the Policy xDS Client example

echo "Starting Policy xDS Client Example..."
echo "Connecting to server at localhost:18000"
echo "Press Ctrl+C to stop"
echo ""

# Wait a bit to ensure server is ready
sleep 2

cd "$(dirname "$0")"
go run main.go
