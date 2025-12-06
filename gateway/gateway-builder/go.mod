module github.com/policy-engine/gateway-builder

go 1.24.0

require (
	github.com/wso2/api-platform/sdk v1.0.0
	golang.org/x/mod v0.30.0
	gopkg.in/yaml.v3 v3.0.1
)

replace github.com/wso2/api-platform/sdk => ../../sdk
