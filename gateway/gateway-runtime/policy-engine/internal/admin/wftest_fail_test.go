package admin

import "testing"

// TEST ONLY: fails on purpose to check that no suite runs after a unit-test failure.
func TestWftestDeliberateFailure(t *testing.T) {
	t.Fatal("deliberate failure for the Platform Tests gate check")
}
