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

package versiontranslator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/dto"
)

func TestDownConvert_RoutesToV1Alpha1(t *testing.T) {
	artifact := &dto.APIDeploymentYAML{ApiVersion: constants.GatewayApiVersion, Kind: constants.RestApi}

	err := DownConvert(constants.RestApi, "v1alpha1", artifact)

	require.NoError(t, err)
	assert.Equal(t, constants.GatewayApiVersionV1Alpha1, artifact.ApiVersion)
}

func TestDownConvert_UnsupportedTargetDataVersion_ReturnsError(t *testing.T) {
	artifact := &dto.APIDeploymentYAML{ApiVersion: constants.GatewayApiVersion}

	err := DownConvert(constants.RestApi, "v2", artifact)

	assert.Error(t, err)
}

func TestKnownKinds_IsNotEmptyAndDefensivelyCopied(t *testing.T) {
	kinds := KnownKinds()
	require.NotEmpty(t, kinds)
	kinds[0] = "mutated"
	assert.NotEqual(t, "mutated", KnownKinds()[0])
}
