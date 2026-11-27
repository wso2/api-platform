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

package dto

import (
	"encoding/json"
	"strings"
	"testing"
)

// errorCode is on the wire only for a refusal, so a gateway's push without one gets the same
// bytes it always did.
func TestImportGatewayArtifactResponse_ErrorCodeOnlyWhenSet(t *testing.T) {
	plain, err := json.Marshal(ImportGatewayArtifactResponse{Status: "failed", Origin: "gateway_api", Error: "boom"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(plain), "errorCode") {
		t.Errorf("errorCode present without a refusal: %s", plain)
	}

	refused, err := json.Marshal(ImportGatewayArtifactResponse{Status: "failed", Origin: "gateway_api",
		Error: "no room", ErrorCode: "quota_exceeded"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(refused), `"errorCode":"quota_exceeded"`) {
		t.Errorf("refusal missing errorCode: %s", refused)
	}
}
