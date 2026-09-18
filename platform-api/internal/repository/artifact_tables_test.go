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

package repository

import (
	"testing"

	"github.com/wso2/api-platform/platform-api/internal/constants"
)

// TestAgentProxyKindIsAValidArtifactKind covers the first of the two independent
// registrations a new artifact kind needs. constants.ValidArtifactKinds gates the
// api-portal webhook's data.api.type and the subscription handler's kind check;
// a kind missing here is rejected there with no other signal.
//
// The second registration — an ArtifactTableRegistry entry for agent_proxies,
// which is what makes the kind visible to the UNION queries behind artifact
// listing, API keys, subscriptions and deployment listing — lands with the
// agent_proxies DDL rather than here. Registering the table before it exists
// makes every one of those UNION queries fail at runtime. The two registrations
// are independent, so this test is extended to assert the registry entry in the
// same commit that creates the table.
func TestAgentProxyKindIsAValidArtifactKind(t *testing.T) {
	if !constants.ValidArtifactKinds[constants.AgentProxy] {
		t.Errorf("constants.ValidArtifactKinds is missing %q", constants.AgentProxy)
	}
}
