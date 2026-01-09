#!/usr/bin/env bash
# Run tests and produce a short summary report saved to test/unit/logs/test-results.log
set -o pipefail

# Ensure script runs from the cli/src directory regardless of invocation cwd
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR/.." || { echo "Failed to change directory to $SCRIPT_DIR/.." >&2; exit 1; }

mkdir -p test/unit/logs

echo "==============================================="
echo "Running Tests"
echo "==============================================="

# Run tests and capture output and exit status
go test -v ./test/unit/... 2>&1 | tee test/unit/logs/test-results.log
TEST_EXIT_CODE=${PIPESTATUS[0]}

# Improve readability by inserting a visual divider before each test run marker
awk 'BEGIN{first=1} /^=== RUN/ { if(!first) print ""; if(!first) print "--------------------------------------------------"; print ""; print; first=0; next } {print}' test/unit/logs/test-results.log > test/unit/logs/test-results.formatted.log || true
# Replace original log with formatted version
mv test/unit/logs/test-results.formatted.log test/unit/logs/test-results.log || true 

echo ""
echo "==============================================="
echo "Test Results Summary"
echo "==============================================="
printf "%-40s | %s\n" "Test Name" "Status"
printf "%-40s-+---------\n" "----------------------------------------"
grep -E "^--- PASS|^--- FAIL" test/unit/logs/test-results.log | \
    awk '/^--- PASS/ { printf "%-40s | ✓ PASS\n", $3 } \
    /^--- FAIL/ { printf "%-40s | ✗ FAIL\n", $3 }' || true

echo "==============================================="
echo "ℹ Full test logs: test/unit/logs/test-results.log"

# Check both test exit code and fail markers in log
if [ $TEST_EXIT_CODE -ne 0 ] || grep -q "^--- FAIL" test/unit/logs/test-results.log 2>/dev/null; then
    echo "✗ Tests failed - build aborted. Check logs for details"
    exit 1
fi

exit 0
