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

package helm

import (
	"testing"
)

func TestDeepMergeValues_probeReplacesInsteadOfMergingHandlers(t *testing.T) {
	base := map[string]interface{}{
		"deployment": map[string]interface{}{
			"livenessProbe": map[string]interface{}{
				"exec": map[string]interface{}{
					"command": []interface{}{"health-check.sh"},
				},
				"initialDelaySeconds": float64(30),
			},
		},
	}
	override := map[string]interface{}{
		"deployment": map[string]interface{}{
			"livenessProbe": map[string]interface{}{
				"httpGet": map[string]interface{}{
					"path": "/health",
					"port": "pe-admin",
				},
				"initialDelaySeconds": float64(5),
			},
		},
	}
	out := deepMergeValues(base, override)
	deploy, _ := out["deployment"].(map[string]interface{})
	probe, _ := deploy["livenessProbe"].(map[string]interface{})
	if _, hasExec := probe["exec"]; hasExec {
		t.Fatalf("expected exec removed after probe merge, got %#v", probe)
	}
	hg, ok := probe["httpGet"].(map[string]interface{})
	if !ok || hg["path"] != "/health" {
		t.Fatalf("expected httpGet from override, got %#v", probe)
	}
}
