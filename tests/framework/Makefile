# Build and run targets for the v2 integration framework.
#
# The testbench and A2A trip-planner images are rebuilt only when their source changes, which is
# why each is a separate target rather than a test dependency. The test-api-portal* targets run
# the API Portal blocks; the API Portal release workflow calls test-api-portal.

TESTBENCH_IMAGE ?= ghcr.io/wso2/api-platform/testbench:test
A2A_TRIP_PLANNER_IMAGE ?= ghcr.io/wso2/api-platform/a2a-trip-planner:test

.PHONY: help testbench a2a-trip-planner ap-cli coverage-report standards-check snapshots snapshots-check \
        test-api-portal test-api-portal-it test-api-portal-ui

# API Portal blocks, run by the API Portal release (make test-api-portal). Keep these lists in
# step with the block files (CLAUDE.md, "Integration Suite Blocks"): every block of
# suites/it/blocks/api-portal.yaml, and every api-portal-* block of suites/ui/ui-suite.yaml.
# A block missing here is not run at release.
API_PORTAL_IT_BLOCKS ?= api-portal-core,api-portal-webhooks,api-portal-auth-scope,api-portal-auth-role,api-portal-grant-table,api-portal-multi-organization,api-portal-foreign-org,api-portal-billing
API_PORTAL_UI_BLOCKS ?= api-portal-core,api-portal-key-manager,api-portal-billing

help: ## Show available targets
	@grep -E '^[a-z-]+:.*##' $(MAKEFILE_LIST) | awk -F':.*##' '{printf "  %-12s %s\n", $$1, $$2}'

testbench: ## Build the testbench image (run after changing testbench/**)
	docker build -f testbench/Dockerfile -t $(TESTBENCH_IMAGE) .

a2a-trip-planner: ## Build the A2A trip-planner agent image used as the Agent proxy upstream
	docker build -t $(A2A_TRIP_PLANNER_IMAGE) ../mock-servers/a2a-trip-planner

ap-cli: ## Build the ap CLI binary the ai-workspace-cli block drives (lands at cli/src/build/ap)
	$(MAKE) -C ../../cli/src build-skip-tests

coverage-report: ## Merge counters from a -coverage run and render txt + HTML reports
	./tools/coverage-report.sh


standards-check: ## Check step and feature isolation rules
	go run ./cmd/standards -root . -unit-root . -steps suites/it/steps -features suites/it/features -scripts tools -suites suites/it/it-suite.yaml,suites/ui/ui-suite.yaml -docs core
	go run ./cmd/standards -root . -unit-root . -steps suites/cloud/steps -features suites/cloud/features -scripts tools -suites suites/cloud/cloud-suite.yaml,suites/it/it-suite.yaml,suites/ui/ui-suite.yaml -docs core
	go run ./cmd/standards -root "" -unit-root "" -steps "" -features suites/ui/features -scripts "" -suites "" -docs ""

snapshots: ## Generate the embedded gateway fixtures from the repository (not committed; the release does the same)
	go run ./cmd/syncassets

snapshots-check: ## Verify generated gateway fixtures equal the repository files
	go run ./cmd/syncassets -check

test-api-portal: test-api-portal-it test-api-portal-ui ## Run the API Portal integration and UI blocks

test-api-portal-it: testbench ## Run the API Portal integration blocks (API_PORTAL_IT_BLOCKS)
	go test -v ./suites/it -count=1 -timeout=60m -logs -blocks=$(API_PORTAL_IT_BLOCKS)

test-api-portal-ui: testbench ## Run the API Portal UI blocks (API_PORTAL_UI_BLOCKS)
	go test -v ./suites/ui -count=1 -timeout=60m -logs -blocks=$(API_PORTAL_UI_BLOCKS)
