module github.com/policy-engine/policies/ratelimit/v0.1.0

go 1.25.0

require github.com/redis/go-redis/v9 v9.7.0

require (
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/wso2/api-platform/sdk v0.0.0-20251213144259-3982d34a2e1c // indirect
)

replace github.com/wso2/api-platform/sdk => ../../../../sdk
