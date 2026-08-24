/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package transform

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
)

// a2aPolicyDefinitionPath is the in-repo policy this transformer attaches by
// name. It lives in its own Go module, so the two cannot share a constant and the
// coupling has to be asserted against the file instead.
const a2aPolicyDefinitionPath = "../../../system-policies/a2a/policy-definition.yaml"

// jsonSchemaObject is as much of a JSON-Schema node as these assertions read: a
// nested parameter block's own required list and property names.
type jsonSchemaObject struct {
	Required   []string                    `yaml:"required"`
	Properties map[string]jsonSchemaObject `yaml:"properties"`
}

// The transformer names the A2A system policy and its parameters as string
// constants. The policy that reads them is a separate module compiled into the
// runtime, so nothing at build time connects the two: a rename on either side
// produces a managed card route whose chain either has no card policy (the
// transform fails, loudly) or has one that finds no content (a 500 on every card
// request, which is only visible in production).
//
// Reading the definition here is what turns that into a build-time failure.
func TestA2APolicyDefinitionMatchesTheTransformerContract(t *testing.T) {
	raw, err := os.ReadFile(a2aPolicyDefinitionPath)
	require.NoError(t, err, "the A2A system policy is missing from the repository")

	var definition struct {
		Name       string           `yaml:"name"`
		Version    string           `yaml:"version"`
		Parameters jsonSchemaObject `yaml:"parameters"`
	}
	require.NoError(t, yaml.Unmarshal(raw, &definition))

	assert.Equal(t, constants.A2A_SYSTEM_POLICY_NAME, definition.Name)

	// The loader requires a full semantic version, and version resolution picks
	// the latest full version for a name — a major-only value here would make the
	// policy unresolvable and every managed card undeployable.
	assert.Regexp(t, `^v\d+\.\d+\.\d+$`, definition.Version)

	// Agent Card serving is one nested block, not top-level fields, so a second
	// gateway-answered A2A concern can be added beside it. The block itself is
	// optional for that reason; its two fields are not.
	agentCard, present := definition.Parameters.Properties[constants.A2A_POLICY_PARAM_AGENT_CARD]
	require.True(t, present, "the %q parameter block is missing from the policy definition",
		constants.A2A_POLICY_PARAM_AGENT_CARD)
	assert.Empty(t, definition.Parameters.Required,
		"no parameter block may be required at the top level; each job brings its own")

	// Exactly the two fields the transformer writes, both required: a field the
	// transformer does not write would arrive absent, and one it writes that the
	// definition does not declare is a name the policy will not read.
	require.Len(t, agentCard.Properties, 2)
	assert.Contains(t, agentCard.Properties, constants.A2A_POLICY_PARAM_CONTENT)
	assert.Contains(t, agentCard.Properties, constants.A2A_POLICY_PARAM_ETAG)
	assert.ElementsMatch(t,
		[]string{constants.A2A_POLICY_PARAM_CONTENT, constants.A2A_POLICY_PARAM_ETAG},
		agentCard.Required)
}

// The policy must be registered in the system build lock, or the gateway is
// built without it and every managed card fails to deploy — with the failure
// arriving at deploy time rather than at build time, where it belongs.
func TestA2APolicyIsInTheSystemBuildLock(t *testing.T) {
	raw, err := os.ReadFile("../../../system-policies/system-build-lock.yaml")
	require.NoError(t, err)

	var lock struct {
		Policies []struct {
			Name     string `yaml:"name"`
			FilePath string `yaml:"filePath"`
		} `yaml:"policies"`
	}
	require.NoError(t, yaml.Unmarshal(raw, &lock))

	for _, entry := range lock.Policies {
		if entry.Name == constants.A2A_SYSTEM_POLICY_NAME {
			assert.NotEmpty(t, entry.FilePath, "a local system policy needs a filePath entry")
			return
		}
	}
	t.Fatalf("%s is not listed in the system build lock", constants.A2A_SYSTEM_POLICY_NAME)
}
