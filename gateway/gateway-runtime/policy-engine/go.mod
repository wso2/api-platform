module github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine

go 1.26.1

require (
	github.com/andybalholm/brotli v1.2.0
	github.com/envoyproxy/go-control-plane/envoy v1.36.0
	github.com/go-viper/mapstructure/v2 v2.4.0
	github.com/google/cel-go v0.26.1
	github.com/google/uuid v1.6.0
	github.com/knadh/koanf/parsers/toml/v2 v2.2.0
	github.com/knadh/koanf/providers/env v1.1.0
	github.com/knadh/koanf/providers/file v1.2.1
	github.com/knadh/koanf/v2 v2.3.0
	github.com/moesif/moesifapi-go v1.1.5
	github.com/prometheus/client_golang v1.23.2
	github.com/stretchr/testify v1.11.1
	github.com/wso2/api-platform/common v0.0.0-20260326194347-3d85c50eae71
	github.com/wso2/api-platform/sdk/core v0.2.7
	go.opentelemetry.io/otel v1.40.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.39.0
	go.opentelemetry.io/otel/sdk v1.40.0
	go.opentelemetry.io/otel/trace v1.40.0
	go.opentelemetry.io/proto/otlp v1.9.0
	google.golang.org/grpc v1.79.3
	google.golang.org/protobuf v1.36.11
	gopkg.in/yaml.v3 v3.0.1
)

require (
	cel.dev/expr v0.25.1 // indirect
	github.com/antlr4-go/antlr/v4 v4.13.1 // indirect
	github.com/aws/aws-sdk-go-v2 v1.31.0 // indirect
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.6.3 // indirect
	github.com/aws/aws-sdk-go-v2/config v1.27.16 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.17.16 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.16.14 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.3.14 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.6.14 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.8.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/bedrockruntime v1.13.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.11.2 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.11.9 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.20.9 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.24.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.28.10 // indirect
	github.com/aws/smithy-go v1.21.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cenkalti/backoff/v5 v5.0.3 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cncf/xds/go v0.0.0-20251210132809-ee656c7534f5 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/envoyproxy/protoc-gen-validate v1.3.0 // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/golang-jwt/jwt/v5 v5.3.1 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.27.3 // indirect
	github.com/knadh/koanf/maps v0.1.2 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/pelletier/go-toml/v2 v2.2.4 // indirect
	github.com/planetscale/vtprotobuf v0.6.1-0.20240319094008-0393e58bdf10 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.66.1 // indirect
	github.com/prometheus/procfs v0.16.1 // indirect
	github.com/redis/go-redis/v9 v9.17.3 // indirect
	github.com/stoewer/go-strcase v1.3.1 // indirect
	github.com/wso2/gateway-controllers/policies/advanced-ratelimit v1.0.1 // indirect
	github.com/wso2/gateway-controllers/policies/analytics-header-filter v1.0.1 // indirect
	github.com/wso2/gateway-controllers/policies/api-key-auth v1.0.1 // indirect
	github.com/wso2/gateway-controllers/policies/aws-bedrock-guardrail v1.0.1 // indirect
	github.com/wso2/gateway-controllers/policies/azure-content-safety-content-moderation v1.0.1 // indirect
	github.com/wso2/gateway-controllers/policies/basic-auth v1.0.1 // indirect
	github.com/wso2/gateway-controllers/policies/basic-ratelimit v1.0.1 // indirect
	github.com/wso2/gateway-controllers/policies/content-length-guardrail v1.0.1 // indirect
	github.com/wso2/gateway-controllers/policies/cors v1.0.1 // indirect
	github.com/wso2/gateway-controllers/policies/dynamic-endpoint v1.0.1 // indirect
	github.com/wso2/gateway-controllers/policies/json-schema-guardrail v1.0.1 // indirect
	github.com/wso2/gateway-controllers/policies/jwt-auth v1.0.1 // indirect
	github.com/wso2/gateway-controllers/policies/log-message v1.0.1 // indirect
	github.com/wso2/gateway-controllers/policies/mcp-acl-list v1.0.2 // indirect
	github.com/wso2/gateway-controllers/policies/mcp-auth v1.0.1 // indirect
	github.com/wso2/gateway-controllers/policies/token-based-ratelimit v1.0.1 // indirect
	github.com/xeipuuv/gojsonpointer v0.0.0-20180127040702-4e3ac2762d5f // indirect
	github.com/xeipuuv/gojsonreference v0.0.0-20180127040603-bd5ef7bd5415 // indirect
	github.com/xeipuuv/gojsonschema v1.2.0 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.39.0 // indirect
	go.opentelemetry.io/otel/metric v1.40.0 // indirect
	go.yaml.in/yaml/v2 v2.4.2 // indirect
	golang.org/x/exp v0.0.0-20260112195511-716be5621a96 // indirect
	golang.org/x/net v0.51.0 // indirect
	golang.org/x/sync v0.20.0 // indirect
	golang.org/x/sys v0.41.0 // indirect
	golang.org/x/text v0.35.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20260128011058-8636f8732409 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260128011058-8636f8732409 // indirect
)

replace github.com/wso2/api-platform/common => ../../../common
