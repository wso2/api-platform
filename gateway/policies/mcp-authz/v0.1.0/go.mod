module github.com/policy-engine/policies/mcp-authorization

require github.com/wso2/api-platform/sdk v0.3.0

require github.com/golang-jwt/jwt/v5 v5.2.2

require github.com/policy-engine/policies/mcp-auth v0.1.0

replace github.com/policy-engine/policies/mcp-auth => ../../mcp-auth/v0.1.0

go 1.25.1
