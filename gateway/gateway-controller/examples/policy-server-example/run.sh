#!/bin/bash

# Script to run the Policy xDS Server example

echo "Starting Policy xDS Server Example..."
echo "Server will listen on port 18000"
echo "Press Ctrl+C to stop"
echo ""

cd "$(dirname "$0")"
go run main.go
