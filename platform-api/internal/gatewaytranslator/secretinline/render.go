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

// Package secretinline replaces {{ secret "handle" }} placeholders in a
// deployment artifact with their plaintext values, for gateways that cannot
// resolve the placeholders themselves (see gwversion.RequiresInlineSecrets).
//
// It is applied at delivery time only — when a gateway fetches an artifact —
// never before the artifact is stored, so deployments.content always keeps the
// placeholders and artifact_secret_refs keeps protecting the referenced
// secrets. The package knows nothing about kinds or gateway versions.
package secretinline

import (
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/wso2/api-platform/platform-api/internal/constants"
)

// Resolver returns the plaintext value for a secret handle.
type Resolver func(handle string) (string, error)

// Render returns content with every {{ secret "handle" }} placeholder replaced
// by resolve(handle). Content without a placeholder is returned as-is,
// byte-identical.
//
// The document is rewritten through the YAML node tree rather than by text
// substitution: only scalar values are touched (never mapping keys), a scalar
// may hold several placeholders, and every changed scalar is re-emitted
// double-quoted so a value containing quotes, '#', ': ', a newline or a
// leading '*', '&', '-' or '!' cannot change the document's structure. Key
// order is preserved. Each handle is resolved once per call; the first
// resolution failure aborts the render and nothing is returned.
func Render(content []byte, resolve Resolver) ([]byte, error) {
	if !constants.SecretPlaceholderRe.Match(content) {
		return content, nil
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(content, &doc); err != nil {
		return nil, fmt.Errorf("secretinline: parse artifact: %w", err)
	}
	cache := map[string]string{}
	memoised := func(handle string) (string, error) {
		if v, ok := cache[handle]; ok {
			return v, nil
		}
		v, err := resolve(handle)
		if err != nil {
			return "", err
		}
		cache[handle] = v
		return v, nil
	}
	if err := renderNode(&doc, memoised); err != nil {
		return nil, err
	}
	out, err := yaml.Marshal(&doc)
	if err != nil {
		return nil, fmt.Errorf("secretinline: encode artifact: %w", err)
	}
	return out, nil
}

// renderNode walks the tree. Mapping keys (even indices of a mapping's
// Content) are skipped on purpose: a placeholder is only ever a value.
func renderNode(n *yaml.Node, resolve Resolver) error {
	switch n.Kind {
	case yaml.DocumentNode, yaml.SequenceNode:
		for _, child := range n.Content {
			if err := renderNode(child, resolve); err != nil {
				return err
			}
		}
	case yaml.MappingNode:
		for i := 1; i < len(n.Content); i += 2 {
			if err := renderNode(n.Content[i], resolve); err != nil {
				return err
			}
		}
	case yaml.ScalarNode:
		if !constants.SecretPlaceholderRe.MatchString(n.Value) {
			return nil
		}
		rendered, err := renderScalar(n.Value, resolve)
		if err != nil {
			return err
		}
		n.Value = rendered
		n.Tag = "!!str"
		n.Style = yaml.DoubleQuotedStyle
	}
	return nil
}

// renderScalar substitutes every placeholder in value, left to right.
func renderScalar(value string, resolve Resolver) (string, error) {
	var resolveErr error
	out := constants.SecretPlaceholderRe.ReplaceAllStringFunc(value, func(match string) string {
		if resolveErr != nil {
			return match
		}
		sub := constants.SecretPlaceholderRe.FindStringSubmatch(match)
		plaintext, err := resolve(sub[1])
		if err != nil {
			resolveErr = fmt.Errorf("secretinline: resolve secret %q: %w", sub[1], err)
			return match
		}
		return plaintext
	})
	if resolveErr != nil {
		return "", resolveErr
	}
	return out, nil
}
