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

package model

import "fmt"

// LLMProxyAttachment is one provider bound to a proxy. It is the only shape the
// service layer reasons about: whatever a request arrived in, and whatever a row
// was stored in, normalises to a list of these before validation, persistence,
// response building or deployment sees it.
//
// It mirrors the gateway's own attachment type field for field
// (gateway/gateway-controller/pkg/models/llm_proxy_attachments.go), so both
// sides of the wire share one mental model.
type LLMProxyAttachment struct {
	ID          string               `json:"id"`
	Alias       string               `json:"alias,omitempty"`
	IsPrimary   bool                 `json:"isPrimary"`
	Auth        *UpstreamAuth        `json:"auth,omitempty"`
	Transformer *LLMProxyTransformer `json:"transformer,omitempty"`
}

// EffectiveName is the logical upstream name policies use to select this
// provider: its alias when set, otherwise its id.
func (a LLMProxyAttachment) EffectiveName() string {
	if a.Alias != "" {
		return a.Alias
	}
	return a.ID
}

// NormaliseLLMProxyAttachments collapses whichever shape a stored configuration
// uses into one list, primary first and the rest in declaration order.
//
// Ordering the primary first is what makes the two shapes produce identical
// output: the legacy shape has no other order, so a canonical list naming the
// same providers normalises to the same sequence wherever its primary entry sat.
//
// Only rows written before the canonical list carry the legacy fields — every
// write since persists `Providers` alone, so a row migrates on its first write
// and this legacy branch shrinks on its own rather than living indefinitely.
func NormaliseLLMProxyAttachments(config LLMProxyConfig) ([]LLMProxyAttachment, error) {
	if len(config.Providers) > 0 {
		primaryCount := 0
		for _, entry := range config.Providers {
			if entry.IsPrimary {
				primaryCount++
			}
		}
		if primaryCount == 0 {
			return nil, fmt.Errorf("'providers' must mark exactly one entry as primary, but none is marked")
		}
		if primaryCount > 1 {
			return nil, fmt.Errorf("'providers' must mark exactly one entry as primary, but %d are marked",
				primaryCount)
		}

		attachments := make([]LLMProxyAttachment, 0, len(config.Providers))
		for _, entry := range config.Providers {
			if entry.IsPrimary {
				attachments = append(attachments, entry)
			}
		}
		for _, entry := range config.Providers {
			if !entry.IsPrimary {
				attachments = append(attachments, entry)
			}
		}
		return attachments, nil
	}

	if config.Provider == "" {
		return nil, fmt.Errorf("a proxy must declare a provider: set 'providers' or 'provider'")
	}

	attachments := []LLMProxyAttachment{{
		ID:        config.Provider,
		IsPrimary: true,
		Auth:      config.UpstreamAuth,
	}}
	for _, additional := range config.AdditionalProviders {
		attachments = append(attachments, LLMProxyAttachment{
			ID:          additional.ID,
			Alias:       additional.As,
			Transformer: additional.Transformer,
		})
	}
	return attachments, nil
}

// PrimaryLLMProxyAttachment returns the attachment the proxy's provider identity
// is taken from — the FK target, the default upstream, and in the legacy shape
// the `provider` field itself.
func PrimaryLLMProxyAttachment(config LLMProxyConfig) (LLMProxyAttachment, error) {
	attachments, err := NormaliseLLMProxyAttachments(config)
	if err != nil {
		return LLMProxyAttachment{}, err
	}
	// NormaliseLLMProxyAttachments guarantees a non-empty list led by the primary.
	return attachments[0], nil
}

// PrimaryLLMProxyProviderID is the provider id of the primary attachment, or the
// empty string when the configuration declares no provider at all. It exists for
// the many read paths that want only the id and have no useful error to return —
// a list row, a log line — and must not start failing on a malformed stored row.
func PrimaryLLMProxyProviderID(config LLMProxyConfig) string {
	primary, err := PrimaryLLMProxyAttachment(config)
	if err != nil {
		return ""
	}
	return primary.ID
}

// ReferencedLLMProviderIDs is every provider a proxy depends on, in either role.
// The deletion guard and the provider-proxies listing both read
// it, which is what keeps them agreeing: a reference invisible to one would be
// invisible to the other rather than letting a destructive delete through.
func ReferencedLLMProviderIDs(config LLMProxyConfig) []string {
	attachments, err := NormaliseLLMProxyAttachments(config)
	if err != nil {
		// A row we cannot normalise still references whatever it names. Falling
		// back to the raw fields keeps the guard conservative: the cost of a
		// false reference is a refused delete, the cost of a missed one is a
		// broken proxy.
		ids := make([]string, 0, 1+len(config.AdditionalProviders))
		if config.Provider != "" {
			ids = append(ids, config.Provider)
		}
		for _, additional := range config.AdditionalProviders {
			ids = append(ids, additional.ID)
		}
		for _, entry := range config.Providers {
			ids = append(ids, entry.ID)
		}
		return ids
	}
	ids := make([]string, 0, len(attachments))
	for _, attachment := range attachments {
		ids = append(ids, attachment.ID)
	}
	return ids
}
