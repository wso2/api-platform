module github.com/policy-engine/policies/mcp-authentication

go 1.25.1

require github.com/wso2/api-platform/sdk v1.0.0

require github.com/policy-engine/policies/jwtauthentication v0.1.0

require github.com/golang-jwt/jwt/v5 v5.2.2 // indirect

replace github.com/wso2/api-platform/sdk => ../../../../sdk

replace github.com/policy-engine/policies/jwtauthentication => ../../jwt-auth/v0.1.0