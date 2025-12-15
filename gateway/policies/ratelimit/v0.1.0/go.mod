module github.com/policy-engine/policies/ratelimit/v0.1.0

go 1.25.0

require (
	github.com/google/cel-go v0.18.2
	github.com/redis/go-redis/v9 v9.7.0
	github.com/wso2/api-platform/sdk v0.0.0-20251213144259-3982d34a2e1c
)

require (
	github.com/antlr4-go/antlr/v4 v4.13.0 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/stoewer/go-strcase v1.2.0 // indirect
	golang.org/x/exp v0.0.0-20230515195305-f3d0a9c9a5cc // indirect
	golang.org/x/text v0.9.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20230803162519-f966b187b2e5 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20230803162519-f966b187b2e5 // indirect
	google.golang.org/protobuf v1.33.0 // indirect
)

replace github.com/wso2/api-platform/sdk => ../../../../sdk
