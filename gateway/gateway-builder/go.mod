module github.com/policy-engine/gateway-builder

go 1.25.1

require (
	github.com/wso2/api-platform/sdk v0.0.0
	golang.org/x/mod v0.30.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/rogpeppe/go-internal v1.14.1 // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
)

// Local module replacements for Docker builds
replace github.com/wso2/api-platform/sdk => ../../sdk
