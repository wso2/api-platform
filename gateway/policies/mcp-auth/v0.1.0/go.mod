module github.com/policy-engine/policies/mcp-authentication

go 1.25.1

require github.com/wso2/api-platform/sdk v0.3.0

require github.com/policy-engine/policies/jwt-auth v0.1.0

require github.com/golang-jwt/jwt/v5 v5.2.2 // indirect

replace github.com/policy-engine/policies/jwt-auth => ../../jwt-auth/v0.1.0