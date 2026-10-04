# Build targets for the v2 integration framework.
#
# Nothing here runs on every test run. The testbench and A2A trip-planner images are rebuilt only
# when their source changes, which is why each is a separate target rather than a test dependency.

TESTBENCH_IMAGE ?= ghcr.io/wso2/api-platform/testbench:test
A2A_TRIP_PLANNER_IMAGE ?= ghcr.io/wso2/api-platform/a2a-trip-planner:test

.PHONY: help testbench a2a-trip-planner coverage-report taxonomy taxonomy-check standards-check
help: ## Show available targets
	@grep -E '^[a-z-]+:.*##' $(MAKEFILE_LIST) | awk -F':.*##' '{printf "  %-12s %s\n", $$1, $$2}'

testbench: ## Build the testbench image (run after changing testbench/**)
	docker build -f testbench/Dockerfile -t $(TESTBENCH_IMAGE) .

a2a-trip-planner: ## Build the A2A trip-planner agent image used as the Agent proxy upstream
	docker build -t $(A2A_TRIP_PLANNER_IMAGE) ../mock-servers/a2a-trip-planner

coverage-report: ## Merge counters from a -coverage run and render txt + HTML reports
	./tools/coverage-report.sh

taxonomy: ## Validate feature annotations and generate the capability coverage tree
	go run ./cmd/taxonomy -map docs/capability-map.yml -features suites/it/features \
		-suite suites/it/it-suite.yaml \
		-output docs/coverage-tree.md

taxonomy-check: ## Verify capability annotations and the generated coverage tree
	go run ./cmd/taxonomy -map docs/capability-map.yml -features suites/it/features \
		-suite suites/it/it-suite.yaml \
		-output docs/coverage-tree.md -check

standards-check: ## Check step and feature isolation rules
	go run ./cmd/standards -root . -unit-root . -steps suites/it/steps -features suites/it/features -scripts tools -suites suites/it/it-suite.yaml,suites/ui/ui-suite.yaml -docs core
	go run ./cmd/standards -root . -unit-root . -steps suites/cloud/steps -features suites/cloud/features -scripts tools -suites suites/cloud/cloud-suite.yaml,suites/it/it-suite.yaml,suites/ui/ui-suite.yaml -docs core
	go run ./cmd/standards -root "" -unit-root "" -steps "" -features suites/ui/features -scripts "" -suites "" -docs ""
