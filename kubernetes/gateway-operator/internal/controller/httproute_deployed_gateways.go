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

package controller

import (
	"encoding/json"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// deployedGatewayEntry is what the route last deployed to one Gateway.
type deployedGatewayEntry struct {
	Hash   string `json:"hash,omitempty"`
	Handle string `json:"handle,omitempty"`
}

// deployedGateways maps a Gateway key ("namespace/name") to what the route last deployed there.
type deployedGateways map[string]deployedGatewayEntry

func gatewayKeyString(key client.ObjectKey) string {
	return key.Namespace + "/" + key.Name
}

func parseGatewayKey(raw string) (client.ObjectKey, bool) {
	ns, name, ok := strings.Cut(strings.TrimSpace(raw), "/")
	if !ok || ns == "" || name == "" || strings.Contains(name, "/") {
		return client.ObjectKey{}, false
	}
	return client.ObjectKey{Namespace: ns, Name: name}, true
}

// deployedGatewaysFromRoute reads the route's deployment record: the multi-Gateway annotation when
// present and readable, plus the legacy single-Gateway pair. The legacy pair alone is the whole
// record for a route on one Gateway, so an upgraded operator neither redeploys nor loses track of
// existing routes. Merging both also picks up a Gateway an older operator deployed to after a
// downgrade.
func deployedGatewaysFromRoute(route *gatewayv1.HTTPRoute) deployedGateways {
	out := deployedGateways{}
	if route == nil {
		return out
	}
	if raw := strings.TrimSpace(route.Annotations[AnnHTTPRouteDeployedGateways]); raw != "" {
		var parsed map[string]deployedGatewayEntry
		if err := json.Unmarshal([]byte(raw), &parsed); err == nil {
			for k, v := range parsed {
				if key, ok := parseGatewayKey(k); ok {
					out[gatewayKeyString(key)] = v
				}
			}
		}
	}
	if key, ok := lastDeployedParentGatewayKeyFromAnnotation(route); ok {
		if _, seen := out[gatewayKeyString(key)]; !seen {
			out[gatewayKeyString(key)] = deployedGatewayEntry{
				Hash:   route.Annotations[AnnHTTPRouteLastDeployedConfigHash],
				Handle: DefaultHTTPRouteAPIHandle(route),
			}
		}
	}
	return out
}

// deployedGatewaysAnnotations returns the bookkeeping annotation values for a record. A nil value
// means "remove".
//
// A route on one Gateway keeps exactly the legacy pair earlier releases wrote, so upgrading and
// downgrading the operator leave it untouched: an older operator would copy any other annotation
// into the payload it hashes and redeploy every route. The multi-Gateway annotation is written
// only when the legacy pair can't hold the record: several Gateways, or an old handle that still
// has to be removed. With no Gateways the legacy hash is left as is, matching earlier releases.
func deployedGatewaysAnnotations(rec deployedGateways, currentHandle string) map[string]*string {
	out := map[string]*string{}
	switch len(rec) {
	case 0:
		out[AnnHTTPRouteDeployedGateways] = nil
		out[AnnHTTPRouteLastDeployedParentGateway] = nil
		return out
	case 1:
		for k, v := range rec {
			key, hash := k, v.Hash
			out[AnnHTTPRouteLastDeployedParentGateway] = &key
			out[AnnHTTPRouteLastDeployedConfigHash] = &hash
			if v.Handle == "" || v.Handle == currentHandle {
				out[AnnHTTPRouteDeployedGateways] = nil
				return out
			}
		}
	default:
		// An older operator only understands one Gateway, so a multi-Gateway route gets no legacy pair.
		out[AnnHTTPRouteLastDeployedParentGateway] = nil
		out[AnnHTTPRouteLastDeployedConfigHash] = nil
	}
	// json.Marshal sorts map keys, so the value is stable across reconciles.
	raw, _ := json.Marshal(map[string]deployedGatewayEntry(rec))
	s := string(raw)
	out[AnnHTTPRouteDeployedGateways] = &s
	return out
}

func lastDeployedParentGatewayKeyFromAnnotation(route *gatewayv1.HTTPRoute) (client.ObjectKey, bool) {
	if route == nil || route.Annotations == nil {
		return client.ObjectKey{}, false
	}
	return parseGatewayKey(route.Annotations[AnnHTTPRouteLastDeployedParentGateway])
}

// handlesToRemove lists the handles to delete from a Gateway: the recorded one and the current one.
func handlesToRemove(entry deployedGatewayEntry, recorded bool, currentHandle string) []string {
	out := []string{currentHandle}
	if recorded && entry.Handle != "" && entry.Handle != currentHandle {
		out = append(out, entry.Handle)
	}
	return out
}
