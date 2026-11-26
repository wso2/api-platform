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

package gatewayclient

import (
	"testing"

	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v2"
)

func TestBuildBudgetYAML_ShapesTheKind(t *testing.T) {
	out, err := BuildBudgetYAML(ManagementArtifactAPIVersion,
		BudgetPayloadMetadata{
			Name:        "org-daily",
			Labels:      []string{"finance", "org"},
			Annotations: map[string]string{"gateway.api-platform.wso2.com/artifact-id": "abc"},
		},
		map[string]interface{}{"limit": map[string]interface{}{"amount": "15.00", "period": "day", "action": "allow"}})
	require.NoError(t, err)

	var got map[string]interface{}
	require.NoError(t, yaml.Unmarshal(out, &got))
	require.Equal(t, "gateway.api-platform.wso2.com/v1", got["apiVersion"])
	require.Equal(t, "Budget", got["kind"])
	metadata := got["metadata"].(map[interface{}]interface{})
	require.Equal(t, "org-daily", metadata["name"])
	require.Equal(t, []interface{}{"finance", "org"}, metadata["labels"], "labels are a list, not a map")
	require.Equal(t, "15.00", got["spec"].(map[interface{}]interface{})["limit"].(map[interface{}]interface{})["amount"],
		"the amount is a string, so YAML must not read it as a number")
}

func TestBuildBudgetYAML_OmitsEmptyLabelsAndAnnotations(t *testing.T) {
	out, err := BuildBudgetYAML(ManagementArtifactAPIVersion, BudgetPayloadMetadata{Name: "org-daily"}, map[string]interface{}{})
	require.NoError(t, err)

	var got map[string]interface{}
	require.NoError(t, yaml.Unmarshal(out, &got))
	metadata := got["metadata"].(map[interface{}]interface{})
	require.NotContains(t, metadata, "labels")
	require.NotContains(t, metadata, "annotations")
}

func TestBuildBudgetYAML_RequiresAnAPIVersionAndAName(t *testing.T) {
	_, err := BuildBudgetYAML("", BudgetPayloadMetadata{Name: "x"}, nil)
	require.ErrorContains(t, err, "apiVersion")
	_, err = BuildBudgetYAML(ManagementArtifactAPIVersion, BudgetPayloadMetadata{Name: " "}, nil)
	require.ErrorContains(t, err, "name")
}
