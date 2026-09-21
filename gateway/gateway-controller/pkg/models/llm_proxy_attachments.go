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

package models

import (
	"fmt"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
)

// LLMProxyAttachment is one provider bound to a proxy, normalised from whichever
// shape the configuration used: the canonical `providers` list, or the legacy
// `provider` plus `additionalProviders` pair. Attachment, validation and
// template resolution all operate on this type and never on the raw shapes, so
// no behaviour can depend on which one arrived.
type LLMProxyAttachment struct {
	Id          string
	Alias       *string
	Auth        *api.LLMUpstreamAuth
	Transformer *api.LLMProxyTransformer
	IsPrimary   bool

	// SourceIndex is the position this attachment occupied in the configuration
	// as it was authored, before normalisation moved the primary to the front.
	// Validation reports field paths with it, so an error about the third entry
	// a user wrote names the third entry rather than wherever it was reordered
	// to. In the legacy shape it is 0 for the primary and 1-based for each
	// additional provider, which matches how those field paths are built.
	SourceIndex int
}

// EffectiveName is the logical upstream name policies use to select this
// provider: its alias when set, otherwise its id.
func (a LLMProxyAttachment) EffectiveName() string {
	if a.Alias != nil && *a.Alias != "" {
		return *a.Alias
	}
	return a.Id
}

// NormaliseLLMProxyAttachments collapses either provider shape into one list,
// primary first and the rest in declaration order. Ordering the primary first
// is what makes the two shapes produce identical output: the legacy shape has
// no other order, so a canonical list that names the same providers normalises
// to the same sequence regardless of where its primary entry sat.
//
// The rejections here are the ones the contract requires: both shapes at once,
// a canonical list with no primary or several, an empty list, and a proxy that
// declares no provider at all.
func NormaliseLLMProxyAttachments(spec api.LLMProxyConfigData) ([]LLMProxyAttachment, error) {
	hasCanonical := spec.Providers != nil
	hasLegacy := spec.Provider != nil || spec.AdditionalProviders != nil

	if hasCanonical && hasLegacy {
		return nil, fmt.Errorf("only one provider shape may be used: either 'providers' or " +
			"'provider' with 'additionalProviders', not both")
	}

	if hasCanonical {
		entries := *spec.Providers
		if len(entries) == 0 {
			return nil, fmt.Errorf("'providers' must not be empty: a proxy always has at least one provider")
		}

		primaryCount := 0
		for _, entry := range entries {
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

		attachments := make([]LLMProxyAttachment, 0, len(entries))
		for i, entry := range entries {
			if entry.IsPrimary {
				attachments = append(attachments, attachmentFromEntry(entry, i))
			}
		}
		for i, entry := range entries {
			if !entry.IsPrimary {
				attachments = append(attachments, attachmentFromEntry(entry, i))
			}
		}
		return attachments, nil
	}

	if spec.Provider == nil {
		return nil, fmt.Errorf("a proxy must declare a provider: set 'providers' or 'provider'")
	}

	attachments := []LLMProxyAttachment{{
		Id:          spec.Provider.Id,
		Alias:       spec.Provider.As,
		Auth:        spec.Provider.Auth,
		Transformer: spec.Provider.Transformer,
		IsPrimary:   true,
		SourceIndex: 0,
	}}
	if spec.AdditionalProviders != nil {
		for i, additional := range *spec.AdditionalProviders {
			attachments = append(attachments, LLMProxyAttachment{
				Id:          additional.Id,
				Alias:       additional.As,
				Auth:        additional.Auth,
				Transformer: additional.Transformer,
				SourceIndex: i + 1,
			})
		}
	}
	return attachments, nil
}

// PrimaryLLMProxyAttachment returns the attachment the proxy's provider
// identity is taken from — the gateway's provider reference, and in the legacy
// shape the `provider` object itself.
func PrimaryLLMProxyAttachment(spec api.LLMProxyConfigData) (LLMProxyAttachment, error) {
	attachments, err := NormaliseLLMProxyAttachments(spec)
	if err != nil {
		return LLMProxyAttachment{}, err
	}
	// NormaliseLLMProxyAttachments guarantees a non-empty list led by the primary.
	return attachments[0], nil
}

func attachmentFromEntry(entry api.LLMProxyProviderEntry, sourceIndex int) LLMProxyAttachment {
	return LLMProxyAttachment{
		Id:          entry.Id,
		Alias:       entry.Alias,
		Auth:        entry.Auth,
		Transformer: entry.Transformer,
		IsPrimary:   entry.IsPrimary,
		SourceIndex: sourceIndex,
	}
}
