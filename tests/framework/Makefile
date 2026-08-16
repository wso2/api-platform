# Build targets for the v2 integration framework.
#
# Nothing here runs on every test run. The testbench image is rebuilt only when its source
# changes, which is why it is a separate target rather than a test dependency.

TESTBENCH_IMAGE ?= ghcr.io/wso2/api-platform/testbench:test

.PHONY: help testbench coverage-report
help: ## Show available targets
	@grep -E '^[a-z-]+:.*##' $(MAKEFILE_LIST) | awk -F':.*##' '{printf "  %-12s %s\n", $$1, $$2}'

testbench: ## Build the testbench image (run after changing testbench/**)
	docker build -f testbench/Dockerfile -t $(TESTBENCH_IMAGE) .

coverage-report: ## Merge counters from a -coverage run and render txt + HTML reports
	./tools/coverage-report.sh
