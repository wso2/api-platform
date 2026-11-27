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

package v2alpha1

// The v2alpha1 APIGateway is a conversion spoke of the v1 hub (storage) type. The
// operator's conversion webhook (internal/conversionwebhook) serves these
// conversions for the API server.
//
// v1 does not declare spec.adminCredentials, so ConvertTo keeps it in the
// ConversionDataAnnotation on the v1 object and ConvertFrom restores it. Reads and
// updates through v1 or v1alpha1 carry the annotation along unchanged, so they never
// lose the value.

import (
	"encoding/json"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/conversion"

	v1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1"
)

// ConversionDataAnnotation holds, on a stored v1 APIGateway, the JSON of the fields
// that only v2alpha1 declares. The conversion webhook owns it: set or clear the
// fields through v2alpha1 rather than editing the annotation.
const ConversionDataAnnotation = "gateway.api-platform.wso2.com/conversion-data"

// conversionData is the ConversionDataAnnotation payload.
type conversionData struct {
	Spec conversionDataSpec `json:"spec"`
}

type conversionDataSpec struct {
	AdminCredentials *GatewayAdminCredentials `json:"adminCredentials,omitempty"`
}

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

// decodeConversionData parses a ConversionDataAnnotation value.
func decodeConversionData(raw string) (*conversionData, error) {
	var data conversionData
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		return nil, fmt.Errorf("annotation %s is not valid conversion data: %w", ConversionDataAnnotation, err)
	}
	return &data, nil
}

// ConvertTo converts this APIGateway to the v1 hub version.
//
// A set spec.adminCredentials is written to the ConversionDataAnnotation. When it is
// unset, a valid annotation is removed; an annotation that does not parse is kept,
// so the error stays visible on the object instead of being silently discarded.
func (src *APIGateway) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1.APIGateway)
	src.ObjectMeta.DeepCopyInto(&dst.ObjectMeta)
	if err := convertViaJSON(src.Spec, &dst.Spec); err != nil {
		return err
	}
	if err := convertViaJSON(src.Status, &dst.Status); err != nil {
		return err
	}

	if src.Spec.AdminCredentials != nil {
		raw, err := json.Marshal(conversionData{Spec: conversionDataSpec{AdminCredentials: src.Spec.AdminCredentials}})
		if err != nil {
			return fmt.Errorf("marshal %s: %w", ConversionDataAnnotation, err)
		}
		if dst.Annotations == nil {
			dst.Annotations = map[string]string{}
		}
		dst.Annotations[ConversionDataAnnotation] = string(raw)
		return nil
	}
	if raw, ok := dst.Annotations[ConversionDataAnnotation]; ok {
		if _, err := decodeConversionData(raw); err == nil {
			delete(dst.Annotations, ConversionDataAnnotation)
		}
	}
	return nil
}

// ConvertFrom converts the v1 hub version to this APIGateway.
//
// A valid ConversionDataAnnotation becomes spec.adminCredentials and is removed from
// the v2alpha1 view. An annotation that does not parse is left in place with
// spec.adminCredentials unset: conversion never fails on it, because one bad object
// would otherwise break every v2alpha1 list that includes it. The operator refuses
// to program such a gateway (see AdminCredentialsOf).
func (dst *APIGateway) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1.APIGateway)
	src.ObjectMeta.DeepCopyInto(&dst.ObjectMeta)
	if err := convertViaJSON(src.Spec, &dst.Spec); err != nil {
		return err
	}
	if err := convertViaJSON(src.Status, &dst.Status); err != nil {
		return err
	}

	dst.Spec.AdminCredentials = nil
	raw, ok := dst.Annotations[ConversionDataAnnotation]
	if !ok {
		return nil
	}
	data, err := decodeConversionData(raw)
	if err != nil {
		return nil
	}
	dst.Spec.AdminCredentials = data.Spec.AdminCredentials
	delete(dst.Annotations, ConversionDataAnnotation)
	if len(dst.Annotations) == 0 {
		dst.Annotations = nil
	}
	return nil
}
