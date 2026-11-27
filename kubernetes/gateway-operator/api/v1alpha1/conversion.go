/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

// The v1alpha1 types are conversion spokes: each root kind implements the
// conversion.Convertible interface (ConvertTo/ConvertFrom) so the conversion
// webhook can translate them to and from the v1 hub (storage) types.
//
// The v1alpha1 and v1 schemas are field-identical (the promotion changed only
// the version label), so Spec and Status are copied with a JSON round-trip.
// ObjectMeta is deep-copied. TypeMeta (apiVersion/kind) is deliberately
// NOT copied — the conversion machinery sets it on the destination.

import (
	"encoding/json"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/conversion"

	v1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1"
)

// convertViaJSON copies in into out by marshaling to JSON and unmarshaling
// back. It is exact when in and out have identical JSON shapes, which holds
// for the v1alpha1 <-> v1 Spec/Status pairs.
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

// --- RestApi ---

func (src *RestApi) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1.RestApi)
	src.ObjectMeta.DeepCopyInto(&dst.ObjectMeta)
	if err := convertViaJSON(src.Spec, &dst.Spec); err != nil {
		return err
	}
	return convertViaJSON(src.Status, &dst.Status)
}

func (dst *RestApi) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1.RestApi)
	src.ObjectMeta.DeepCopyInto(&dst.ObjectMeta)
	if err := convertViaJSON(src.Spec, &dst.Spec); err != nil {
		return err
	}
	return convertViaJSON(src.Status, &dst.Status)
}

// --- APIGateway ---

func (src *APIGateway) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1.APIGateway)
	src.ObjectMeta.DeepCopyInto(&dst.ObjectMeta)
	if err := convertViaJSON(src.Spec, &dst.Spec); err != nil {
		return err
	}
	return convertViaJSON(src.Status, &dst.Status)
}

func (dst *APIGateway) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1.APIGateway)
	src.ObjectMeta.DeepCopyInto(&dst.ObjectMeta)
	if err := convertViaJSON(src.Spec, &dst.Spec); err != nil {
		return err
	}
	return convertViaJSON(src.Status, &dst.Status)
}

// --- ApiKey ---

func (src *ApiKey) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1.ApiKey)
	src.ObjectMeta.DeepCopyInto(&dst.ObjectMeta)
	if err := convertViaJSON(src.Spec, &dst.Spec); err != nil {
		return err
	}
	return convertViaJSON(src.Status, &dst.Status)
}

func (dst *ApiKey) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1.ApiKey)
	src.ObjectMeta.DeepCopyInto(&dst.ObjectMeta)
	if err := convertViaJSON(src.Spec, &dst.Spec); err != nil {
		return err
	}
	return convertViaJSON(src.Status, &dst.Status)
}

// --- APIPolicy ---

func (src *APIPolicy) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1.APIPolicy)
	src.ObjectMeta.DeepCopyInto(&dst.ObjectMeta)
	if err := convertViaJSON(src.Spec, &dst.Spec); err != nil {
		return err
	}
	return convertViaJSON(src.Status, &dst.Status)
}

func (dst *APIPolicy) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1.APIPolicy)
	src.ObjectMeta.DeepCopyInto(&dst.ObjectMeta)
	if err := convertViaJSON(src.Spec, &dst.Spec); err != nil {
		return err
	}
	return convertViaJSON(src.Status, &dst.Status)
}

// --- Certificate ---

func (src *Certificate) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1.Certificate)
	src.ObjectMeta.DeepCopyInto(&dst.ObjectMeta)
	if err := convertViaJSON(src.Spec, &dst.Spec); err != nil {
		return err
	}
	return convertViaJSON(src.Status, &dst.Status)
}

func (dst *Certificate) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1.Certificate)
	src.ObjectMeta.DeepCopyInto(&dst.ObjectMeta)
	if err := convertViaJSON(src.Spec, &dst.Spec); err != nil {
		return err
	}
	return convertViaJSON(src.Status, &dst.Status)
}

// --- LlmProvider ---

func (src *LlmProvider) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1.LlmProvider)
	src.ObjectMeta.DeepCopyInto(&dst.ObjectMeta)
	if err := convertViaJSON(src.Spec, &dst.Spec); err != nil {
		return err
	}
	return convertViaJSON(src.Status, &dst.Status)
}

func (dst *LlmProvider) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1.LlmProvider)
	src.ObjectMeta.DeepCopyInto(&dst.ObjectMeta)
	if err := convertViaJSON(src.Spec, &dst.Spec); err != nil {
		return err
	}
	return convertViaJSON(src.Status, &dst.Status)
}

// --- LlmProviderTemplate ---

func (src *LlmProviderTemplate) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1.LlmProviderTemplate)
	src.ObjectMeta.DeepCopyInto(&dst.ObjectMeta)
	if err := convertViaJSON(src.Spec, &dst.Spec); err != nil {
		return err
	}
	return convertViaJSON(src.Status, &dst.Status)
}

func (dst *LlmProviderTemplate) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1.LlmProviderTemplate)
	src.ObjectMeta.DeepCopyInto(&dst.ObjectMeta)
	if err := convertViaJSON(src.Spec, &dst.Spec); err != nil {
		return err
	}
	return convertViaJSON(src.Status, &dst.Status)
}

// --- LlmProxy ---

func (src *LlmProxy) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1.LlmProxy)
	src.ObjectMeta.DeepCopyInto(&dst.ObjectMeta)
	if err := convertViaJSON(src.Spec, &dst.Spec); err != nil {
		return err
	}
	return convertViaJSON(src.Status, &dst.Status)
}

func (dst *LlmProxy) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1.LlmProxy)
	src.ObjectMeta.DeepCopyInto(&dst.ObjectMeta)
	if err := convertViaJSON(src.Spec, &dst.Spec); err != nil {
		return err
	}
	return convertViaJSON(src.Status, &dst.Status)
}

// --- ManagedSecret ---

func (src *ManagedSecret) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1.ManagedSecret)
	src.ObjectMeta.DeepCopyInto(&dst.ObjectMeta)
	if err := convertViaJSON(src.Spec, &dst.Spec); err != nil {
		return err
	}
	return convertViaJSON(src.Status, &dst.Status)
}

func (dst *ManagedSecret) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1.ManagedSecret)
	src.ObjectMeta.DeepCopyInto(&dst.ObjectMeta)
	if err := convertViaJSON(src.Spec, &dst.Spec); err != nil {
		return err
	}
	return convertViaJSON(src.Status, &dst.Status)
}

// --- Mcp ---

func (src *Mcp) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1.Mcp)
	src.ObjectMeta.DeepCopyInto(&dst.ObjectMeta)
	if err := convertViaJSON(src.Spec, &dst.Spec); err != nil {
		return err
	}
	return convertViaJSON(src.Status, &dst.Status)
}

func (dst *Mcp) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1.Mcp)
	src.ObjectMeta.DeepCopyInto(&dst.ObjectMeta)
	if err := convertViaJSON(src.Spec, &dst.Spec); err != nil {
		return err
	}
	return convertViaJSON(src.Status, &dst.Status)
}

// --- Subscription ---

func (src *Subscription) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1.Subscription)
	src.ObjectMeta.DeepCopyInto(&dst.ObjectMeta)
	if err := convertViaJSON(src.Spec, &dst.Spec); err != nil {
		return err
	}
	return convertViaJSON(src.Status, &dst.Status)
}

func (dst *Subscription) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1.Subscription)
	src.ObjectMeta.DeepCopyInto(&dst.ObjectMeta)
	if err := convertViaJSON(src.Spec, &dst.Spec); err != nil {
		return err
	}
	return convertViaJSON(src.Status, &dst.Status)
}

// --- SubscriptionPlan ---

func (src *SubscriptionPlan) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1.SubscriptionPlan)
	src.ObjectMeta.DeepCopyInto(&dst.ObjectMeta)
	if err := convertViaJSON(src.Spec, &dst.Spec); err != nil {
		return err
	}
	return convertViaJSON(src.Status, &dst.Status)
}

func (dst *SubscriptionPlan) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1.SubscriptionPlan)
	src.ObjectMeta.DeepCopyInto(&dst.ObjectMeta)
	if err := convertViaJSON(src.Spec, &dst.Spec); err != nil {
		return err
	}
	return convertViaJSON(src.Status, &dst.Status)
}
