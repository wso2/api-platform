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

package v1alpha2

// The v1alpha2 APIGateway is a conversion spoke of the v1 hub (storage) type.
// The two schemas are field-identical, so Spec and Status are copied with a JSON
// round-trip, as in api/v1alpha1/conversion.go. The CRD serves its versions with
// conversion strategy None, so the API server only relabels apiVersion; this
// scaffolding is not wired to a live webhook.

import (
	"encoding/json"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/conversion"

	v1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1"
)

func convertViaJSON(in, out any) error {
	data, err := json.Marshal(in)
	if err != nil {
		return fmt.Errorf("marshal for conversion: %w", err)
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("unmarshal for conversion: %w", err)
	}
	return nil
}

// ConvertTo converts this APIGateway to the v1 hub version.
func (src *APIGateway) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1.APIGateway)
	dst.ObjectMeta = src.ObjectMeta
	if err := convertViaJSON(src.Spec, &dst.Spec); err != nil {
		return err
	}
	return convertViaJSON(src.Status, &dst.Status)
}

// ConvertFrom converts the v1 hub version to this APIGateway.
func (dst *APIGateway) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1.APIGateway)
	dst.ObjectMeta = src.ObjectMeta
	if err := convertViaJSON(src.Spec, &dst.Spec); err != nil {
		return err
	}
	return convertViaJSON(src.Status, &dst.Status)
}
