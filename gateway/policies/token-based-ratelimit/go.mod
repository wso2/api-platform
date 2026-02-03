module github.com/wso2/gateway-controllers/policies/token-based-ratelimit

go 1.25.1

replace github.com/wso2/gateway-controllers/policies/advanced-ratelimit => ../advanced-ratelimit

require (
	github.com/wso2/api-platform/sdk v0.3.8
	github.com/wso2/gateway-controllers/policies/advanced-ratelimit v0.1.3
	golang.org/x/sync v0.19.0
)

require (
	cel.dev/expr v0.25.1 // indirect
	github.com/antlr4-go/antlr/v4 v4.13.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/google/cel-go v0.26.1 // indirect
	github.com/redis/go-redis/v9 v9.17.3 // indirect
	github.com/stoewer/go-strcase v1.3.1 // indirect
	golang.org/x/crypto v0.47.0 // indirect
	golang.org/x/exp v0.0.0-20260112195511-716be5621a96 // indirect
	golang.org/x/sys v0.40.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20260128011058-8636f8732409 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260128011058-8636f8732409 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)
