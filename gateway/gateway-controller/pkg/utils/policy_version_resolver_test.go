package utils

import "github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"

const (
	testSetHeadersVersion = "v9.9.9"
	testRespondVersion    = "v9.9.8"
)

func newTestPolicyVersionResolver() PolicyVersionResolver {
	return NewStaticPolicyVersionResolver(map[string]string{
		constants.UPSTREAM_AUTH_APIKEY_POLICY_NAME: testSetHeadersVersion,
		constants.ACCESS_CONTROL_DENY_POLICY_NAME:  testRespondVersion,
	})
}
