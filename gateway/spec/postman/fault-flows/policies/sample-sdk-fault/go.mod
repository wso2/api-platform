module github.com/wso2/gateway-controllers/policies/sample-sdk-fault

go 1.26.2

require github.com/wso2/api-platform/sdk/core v0.4.2

// Built from this repository's SDK, which carries the fault contract.
replace github.com/wso2/api-platform/sdk/core => ../../../sdk/core
