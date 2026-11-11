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

package apikey

import (
	"github.com/wso2/api-platform/cli/cmd/gateway/apikeycmd"
	"github.com/wso2/api-platform/cli/utils"
)

// apiKeyConfig parameterizes the shared api-key request logic and root-group
// builder (see cmd/gateway/apikeycmd) for SOAP APIs. create.go/update.go
// are not templated - see apikeycmd's package doc for why.
var apiKeyConfig = apikeycmd.Config{
	KindLabel:       "SOAP API",
	KindPathSegment: "soap-api",
	ExampleAPIID:    "stockquote-v1.0",
	KeysPath:        utils.GatewaySoapAPIKeysPath,
	KeyByNamePath:   utils.GatewaySoapAPIKeyByNamePath,
	RegeneratePath:  utils.GatewaySoapAPIKeyRegeneratePath,
	CreateExample:   "# Generate a new API key with an auto-generated name\nap gateway soap-api api-key create --id stockquote-v1.0",
}

// APIKeyCmd represents the gateway SOAP API api-key command group. API keys
// are scoped to a SOAP API via the /soap-apis/{id}/api-keys management
// endpoints.
var APIKeyCmd = apikeycmd.NewRootCmd(apiKeyConfig, createCmd, listCmd, regenerateCmd, updateCmd, revokeCmd)
