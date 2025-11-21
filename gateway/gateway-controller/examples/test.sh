#!/bin/bash

# Quick test script to verify Policy xDS implementation

set -e

echo "======================================"
echo "Policy xDS Implementation Test"
echo "======================================"
echo ""

# Build both examples
echo "📦 Building examples..."
echo ""

echo "  Building server..."
cd "$(dirname "$0")/policy-server-example"
go build -o server main.go
echo "  ✅ Server built successfully"

echo "  Building client..."
cd ../policy-client-example
go build -o client main.go
echo "  ✅ Client built successfully"

echo ""
echo "======================================"
echo "Build completed successfully!"
echo "======================================"
echo ""
echo "To test the implementation:"
echo ""
echo "1. In Terminal 1, run the server:"
echo "   cd examples/policy-server-example"
echo "   ./run.sh"
echo ""
echo "2. In Terminal 2, run the client:"
echo "   cd examples/policy-client-example"
echo "   ./run.sh"
echo ""
echo "You should see:"
echo "  ✅ Client connects to server"
echo "  ✅ Client receives initial policies"
echo "  ✅ Client receives dynamic updates every 10s"
echo ""
