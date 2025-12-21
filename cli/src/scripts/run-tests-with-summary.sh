#!/usr/bin/env bash
# Run tests and produce a short summary report saved to tests/logs/test-results.log
set -o pipefail

mkdir -p tests/logs

echo "==============================================="
echo "Running Tests"
echo "==============================================="

# Run tests and capture output
go test -v ./tests/... 2>&1 | tee tests/logs/test-results.log

echo ""
echo "==============================================="
echo "Test Results Summary"
echo "==============================================="
printf "%-40s | %s\n" "Test Name" "Status"
printf "%-40s-+---------\n" "----------------------------------------"
grep -E "^=== RUN|^--- PASS|^--- FAIL" tests/logs/test-results.log | \
    awk 'BEGIN { test="" } \
    /^=== RUN/ { test=$3 } \
    /^--- PASS/ { printf "%-40s | ✓ PASS\n", test } \
    /^--- FAIL/ { printf "%-40s | ✗ FAIL\n", test }' || true

echo "==============================================="
echo "ℹ Full test logs: tests/logs/test-results.log"

if grep -q "^--- FAIL" tests/logs/test-results.log 2>/dev/null; then
    echo "✗ Tests failed - build aborted. Check logs for details"
    exit 1
fi

exit 0
