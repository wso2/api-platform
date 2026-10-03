/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package service

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/wso2/api-platform/platform-api/internal/model"
)

func TestSanitizeDeploymentStatusReason(t *testing.T) {
	cases := []struct {
		name, raw, want string
	}{
		// Well-formed codes are kept as sent, whichever kind sent them, and an
		// empty code stays empty (stored as NULL).
		{"empty", "", ""},
		{"gateway fault", model.DeploymentErrorGatewayFailure, model.DeploymentErrorGatewayFailure},
		{"agent validation", model.DeploymentErrorAgentValidationFailed, model.DeploymentErrorAgentValidationFailed},
		{"agent render", model.DeploymentErrorAgentRenderFailed, model.DeploymentErrorAgentRenderFailed},
		{"websub code", "WEBSUBHUB_INTERNAL_CLUSTER", "WEBSUBHUB_INTERNAL_CLUSTER"},
		{"exactly the column width", strings.Repeat("A", maxStatusReasonLen), strings.Repeat("A", maxStatusReasonLen)},
		// Anything that is not a code is never persisted or served.
		{"too long for the column", strings.Repeat("A", maxStatusReasonLen+1), model.DeploymentErrorGatewayFailure},
		{"free text", "connection refused: password=hunter2", model.DeploymentErrorGatewayFailure},
		{"leading digit", "1_BAD", model.DeploymentErrorGatewayFailure},
		{"embedded newline", "BAD\nCODE", model.DeploymentErrorGatewayFailure},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, sanitizeDeploymentStatusReason(tc.raw))
		})
	}
}
