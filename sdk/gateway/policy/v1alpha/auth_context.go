package policyv1alpha

import core "github.com/wso2/api-platform/sdk/core/policy"

// AuthContext contains structured authentication information populated by auth policies.
// It replaces the unstructured map[string]string previously used for inter-policy
// communication about authentication results.
type AuthContext = core.AuthContext
