#!/bin/bash

# STS Startup Script - Phase 3
# Starts Thunder OAuth 2.0 / OIDC server and Gate App authentication UI

set -e

echo "========================================"
echo "Starting STS (Security Token Service)"
echo "========================================"
echo ""

echo "[1/2] Starting Thunder OAuth 2.0 / OIDC Server..."
echo "      Thunder will be available on port 8090"
echo ""

# Start Thunder in the background
cd /opt/thunder
./start.sh &

# Store Thunder PID
THUNDER_PID=$!
echo "      Thunder started (PID: $THUNDER_PID)"
echo ""

# Wait a bit for Thunder to initialize
sleep 5

echo "[2/2] Starting Gate App Authentication UI..."
echo "      Gate App will be available on port 9091"
echo ""

# Start Gate App in the foreground
cd /opt/gate-app
exec node server.js
