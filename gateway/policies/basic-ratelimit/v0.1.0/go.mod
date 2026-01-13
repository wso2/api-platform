module github.com/policy-engine/policies/basic-ratelimit

go 1.25.1

require (
	github.com/policy-engine/policies/advanced-ratelimit v0.1.0
	github.com/wso2/api-platform/sdk v0.3.0
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/redis/go-redis/v9 v9.17.2 // indirect
)

replace (
	github.com/policy-engine/policies/advanced-ratelimit => ../../advanced-ratelimit/v0.1.0
	github.com/wso2/api-platform/sdk => ../../../../sdk
)
