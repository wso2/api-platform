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

package config

import (
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
)

// Agent Card configuration is optional at three levels — the whole `agentCard`
// block, its `public` block, and that block's `mode` — and every consumer has to
// read the same defaults out of the same silence. This file is where those
// defaults live, and it is the only place they are written down.
//
// The reason it is one shared helper rather than a default applied per consumer
// is the failure mode. The validator, the transformer, the policy validator and
// the reload/event path each read this configuration independently; a
// disagreement between any two of them is not a validation error, it is an Agent
// that deployed cleanly and then behaves as something the author did not
// configure — a card served at a path the collision check never considered, or a
// route generated for a representation validation never saw. An OpenAPI
// `default:` does not help here: `oapi-codegen` emits a pointer field and leaves
// it nil, so the default exists in the document and nowhere in the Go code.
//
// The public and protected representations default *differently*, and that
// asymmetry is deliberate rather than an oversight. See EffectivePublicCard and
// EffectiveProtectedCardMode.

// PublicCardConfig is the public Agent Card configuration with the defaults
// of an omitted block, an omitted `public` block, and an omitted `mode` already
// applied.
//
// The `…Stated` flags record whether the author wrote the field out, which is
// not the same question as what its effective value is. Validation needs both:
// the effective value decides what routes and chains get built, while
// "was it stated" decides whether a rule about writing a field in the wrong mode
// has been broken at all — `rewriteUrls: false` under `mode: managed` is still a
// field that does not belong there, and reporting it only when true would accept
// a configuration whose author believed the flag meant something.
type PublicCardConfig struct {
	// Mode is the resolved production mode: what the author wrote, or
	// passthrough.
	Mode api.A2APublicAgentCardMode
	// ModeStated reports whether `mode` was written out at all. An empty stated
	// mode is a validation error rather than an omission, so this distinguishes
	// the two.
	ModeStated bool

	// Path is the card path relative to the Agent's context, defaulted to the
	// location A2A clients probe during cold discovery. Never empty: an
	// explicitly empty path is a validation error, and until that rejection
	// happens this resolves to the default rather than to a route at the
	// context itself.
	Path string
	// StatedPath is the value the author wrote, which is what a rejection has to
	// name — including when it is the empty string, the one stated value Path
	// cannot carry. Meaningful only when PathStated is true.
	StatedPath string
	// PathStated reports whether `path` was written out, so a rejection can name
	// the author's own value rather than the default they never chose.
	PathStated bool

	// RewriteUrls reports whether a proxied card response's interface URLs are
	// rewritten to the gateway's own endpoints. Defaults to enabled — see
	// EffectiveRewriteUrls.
	RewriteUrls bool
	// RewriteUrlsStated reports whether the flag was written out, in either
	// polarity.
	RewriteUrlsStated bool

	// Policies, Content and Signing are passed through untouched. Nothing is
	// defaulted into them: an absent policy list is no policies, and an absent
	// document or signing block is exactly what the mode rules already reason
	// about.
	Policies *[]api.Policy
	Content  *api.A2AAgentCardDocument
	Signing  *api.A2ACardSigning
}

// EffectivePublicCard resolves the public Agent Card configuration, including
// the case where the Agent wrote no card configuration at all.
//
// Passthrough is the default because the public card is a discovery document the
// upstream agent already serves: an Agent that says nothing about its card gets
// the card its own agent publishes, proxied at the well-known path, with the
// gateway neither parsing nor rewriting it. The alternative default — managed —
// is not expressible from silence, since it requires a document the author has
// to supply.
//
// Note the asymmetry with the protected representation, which also defaults to
// passthrough but for the opposite reason: there, passthrough is the *guarded*
// reading of silence (see EffectiveProtectedCardMode), and an omitted protected
// block is deliberately never turned into an explicit one — the two blocks are
// resolved by separate helpers so that difference cannot be lost in a shared
// one.
func EffectivePublicCard(card *api.A2AAgentCard) PublicCardConfig {
	effective := PublicCardConfig{
		Mode:        api.A2APublicAgentCardModePassthrough,
		Path:        DefaultAgentCardPath,
		RewriteUrls: EffectiveRewriteUrls(nil),
	}
	if card == nil || card.Public == nil {
		return effective
	}

	public := card.Public
	if public.Mode != nil {
		effective.Mode = *public.Mode
		effective.ModeStated = true
	}
	if public.Path != nil {
		effective.StatedPath = *public.Path
		effective.PathStated = true
		if *public.Path != "" {
			effective.Path = *public.Path
		}
	}
	if public.RewriteUrls != nil {
		effective.RewriteUrls = EffectiveRewriteUrls(public.RewriteUrls)
		effective.RewriteUrlsStated = true
	}
	effective.Policies = public.Policies
	effective.Content = public.Content
	effective.Signing = public.Signing
	return effective
}

// ProtectedCard is the explicitly configured protected Agent Card block, or nil.
//
// It is a nil-safe accessor rather than a defaulting one, and that is the whole
// point: an omitted block keeps the compatibility behaviour it shipped with, and
// synthesising a block here would erase the difference between "the author
// configured passthrough" and "the author said nothing" for every caller
// downstream. What an absent block *means* is resolved where it is needed —
// EffectiveProtectedCardMode for the mode, EffectiveRewriteUrls for the flag —
// rather than by inventing configuration the author did not write.
func ProtectedCard(card *api.A2AAgentCard) *api.A2AProtectedAgentCard {
	if card == nil {
		return nil
	}
	return card.Protected
}

// EffectiveRewriteUrls resolves a per-representation `rewriteUrls` flag.
//
// The default is true in both representations. A proxied card is the agent's own
// discovery document, advertising the URLs the agent is reachable at — so
// forwarding it unchanged hands every client the address of the agent behind the
// gateway, and those clients then bypass the gateway and every policy on it.
// Defaulting to off would make that the behaviour an author gets by saying
// nothing, which is the opposite of why the gateway is in front of the agent.
//
// The cost of the default is real and is why the flag exists in both polarities:
// rewriting drops the card's `signatures` block, which no longer covers the
// returned bytes and which the gateway will not re-sign. An author who needs the
// upstream's signed document delivered intact sets the flag to false and accepts
// that clients configured from it will not traverse the gateway.
func EffectiveRewriteUrls(flag *api.A2ACardRewriteUrls) bool {
	return flag == nil || *flag
}
