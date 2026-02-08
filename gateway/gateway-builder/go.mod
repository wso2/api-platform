module github.com/wso2/api-platform/gateway/gateway-builder

go 1.25.1

require (
	github.com/stretchr/testify v1.11.1
	github.com/wso2/api-platform/sdk v0.0.0
	golang.org/x/mod v0.30.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	golang.org/x/crypto v0.46.0 // indirect
	golang.org/x/sys v0.39.0 // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
)

// Local module replacements for Docker builds
replace github.com/wso2/api-platform/sdk => ../../sdk
