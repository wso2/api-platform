#!/bin/bash

# STS Startup Script - Phase 1
# Starts Thunder OAuth 2.0 / OIDC server

set -e

echo "========================================"
echo "Starting STS (Security Token Service)"
echo "========================================"
echo ""

echo "[Phase 1] Starting Thunder OAuth 2.0 / OIDC Server..."
echo "Thunder will be available on port 8090"
echo ""

# Change to Thunder directory and start
cd /opt/thunder

# Start Thunder
exec ./start.sh
