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

package topology

import (
	"fmt"
	"strconv"
	"strings"
)

const gatewayVersionPrefix = "gateway-version"

type gatewayReleaseVersion struct {
	major uint64
	minor uint64
	patch uint64
}

func (v gatewayReleaseVersion) compare(other gatewayReleaseVersion) int {
	for _, pair := range [][2]uint64{{v.major, other.major}, {v.minor, other.minor}, {v.patch, other.patch}} {
		switch {
		case pair[0] < pair[1]:
			return -1
		case pair[0] > pair[1]:
			return 1
		}
	}
	return 0
}

// String returns the canonical release form of the Gateway version.
func (v gatewayReleaseVersion) String() string {
	return fmt.Sprintf("%d.%d.%d", v.major, v.minor, v.patch)
}

type gatewayVersionConstraint struct {
	operator string
	version  gatewayReleaseVersion
}

// String returns the runner-selector form of the compatibility constraint.
func (c gatewayVersionConstraint) String() string {
	return gatewayVersionPrefix + c.operator + c.version.String()
}

func (c gatewayVersionConstraint) matches(version gatewayReleaseVersion) bool {
	comparison := version.compare(c.version)
	switch c.operator {
	case ">":
		return comparison > 0
	case ">=":
		return comparison >= 0
	case "<":
		return comparison < 0
	case "<=":
		return comparison <= 0
	case "=", "==":
		return comparison == 0
	default:
		return false
	}
}

// matchesSource treats a source build as the current development capability set. Release-only
// compatibility runners are intentionally excluded from it, while runners requiring a version
// newer than an older release remain enabled.
func (c gatewayVersionConstraint) matchesSource() bool {
	return c.operator == ">" || c.operator == ">="
}

type gatewayVersionTarget struct {
	release *gatewayReleaseVersion
	source  bool
}

// String returns the release or source-build label used in selection diagnostics.
func (t gatewayVersionTarget) String() string {
	if t.source {
		return "current source build"
	}
	if t.release == nil {
		return "<unknown>"
	}
	return t.release.String()
}

func (t gatewayVersionTarget) matches(constraint gatewayVersionConstraint) bool {
	if t.source {
		return constraint.matchesSource()
	}
	return t.release != nil && constraint.matches(*t.release)
}

func selectGatewayVersionRunners(block *ResolvedBlock) ([]Runner, []SkippedRunner, error) {
	if block == nil {
		return nil, nil, fmt.Errorf("topology: resolved block is required")
	}
	hasConstraint := false
	for _, runner := range block.Runners {
		if runner.GatewayVersion != nil {
			hasConstraint = true
			break
		}
	}
	if !hasConstraint {
		return block.Runners, nil, nil
	}

	target, err := gatewayVersionFor(block)
	if err != nil {
		return nil, nil, err
	}
	runners := make([]Runner, 0, len(block.Runners))
	skipped := make([]SkippedRunner, 0, len(block.Runners))
	for _, runner := range block.Runners {
		if runner.GatewayVersion == nil || target.matches(*runner.GatewayVersion) {
			runners = append(runners, runner)
			continue
		}
		skipped = append(skipped, SkippedRunner{
			Block:  block.Name,
			Runner: runner.Name,
			Reason: fmt.Sprintf("Gateway version %s does not satisfy %s", target, runner.GatewayVersion),
		})
	}
	return runners, skipped, nil
}

func gatewayVersionFor(block *ResolvedBlock) (gatewayVersionTarget, error) {
	for _, component := range block.Components {
		if component.Def == nil || component.Def.Name != "platform-gateway" {
			continue
		}
		if component.BuildFromSource {
			return gatewayVersionTarget{source: true}, nil
		}
		version, err := parseGatewayReleaseVersion(component.Version)
		if err != nil {
			return gatewayVersionTarget{}, fmt.Errorf("topology: block %q platform-gateway version %q: %w",
				block.Name, component.Version, err)
		}
		return gatewayVersionTarget{release: &version}, nil
	}
	return gatewayVersionTarget{}, fmt.Errorf("topology: block %q uses %s but does not declare platform-gateway",
		block.Name, gatewayVersionPrefix)
}

func parseRunnerTags(block string, runners []Runner) ([]Runner, error) {
	parsed := make([]Runner, len(runners))
	copy(parsed, runners)
	for i := range parsed {
		godogTags, constraint, err := parseGatewayVersionTag(parsed[i].Tags)
		if err != nil {
			return nil, fmt.Errorf("block %q runner %q: %w", block, parsed[i].Name, err)
		}
		parsed[i].Tags = godogTags
		parsed[i].GatewayVersion = constraint
	}
	return parsed, nil
}

// parseGatewayVersionTag separates an optional framework compatibility selector from the
// Godog expression. A semicolon is reserved as the separator because it is not part of Godog's
// tag-expression grammar.
func parseGatewayVersionTag(raw string) (string, *gatewayVersionConstraint, error) {
	parts := strings.Split(raw, ";")
	if len(parts) == 1 {
		if !strings.HasPrefix(strings.TrimSpace(parts[0]), gatewayVersionPrefix) {
			return raw, nil, nil
		}
		constraint, err := parseGatewayVersionConstraint(parts[0])
		if err != nil {
			return "", nil, err
		}
		return "", &constraint, nil
	}
	if len(parts) != 2 {
		return "", nil, fmt.Errorf("tags may contain at most one ';' separator")
	}

	selector := parts[0]
	if !strings.HasPrefix(selector, gatewayVersionPrefix) {
		return "", nil, fmt.Errorf("the clause before ';' must be a %s comparison", gatewayVersionPrefix)
	}
	constraint, err := parseGatewayVersionConstraint(selector)
	if err != nil {
		return "", nil, err
	}
	godogTags := strings.TrimSpace(parts[1])
	if godogTags == "" {
		return "", nil, fmt.Errorf("Godog tags after ';' must not be empty")
	}
	return godogTags, &constraint, nil
}

func parseGatewayVersionConstraint(raw string) (gatewayVersionConstraint, error) {
	if strings.TrimSpace(raw) != raw {
		return gatewayVersionConstraint{}, fmt.Errorf("%s comparison must not contain surrounding whitespace", gatewayVersionPrefix)
	}
	remaining := strings.TrimPrefix(raw, gatewayVersionPrefix)
	if remaining == raw {
		return gatewayVersionConstraint{}, fmt.Errorf("expected a %s comparison", gatewayVersionPrefix)
	}
	for _, operator := range []string{"<=", ">=", "==", ">", "<", "="} {
		if !strings.HasPrefix(remaining, operator) {
			continue
		}
		version, err := parseGatewayReleaseVersion(strings.TrimPrefix(remaining, operator))
		if err != nil {
			return gatewayVersionConstraint{}, err
		}
		return gatewayVersionConstraint{operator: operator, version: version}, nil
	}
	return gatewayVersionConstraint{}, fmt.Errorf("%s comparison requires one of >, >=, <, <=, =, or ==", gatewayVersionPrefix)
}

func parseGatewayReleaseVersion(raw string) (gatewayReleaseVersion, error) {
	original := raw
	raw = strings.TrimPrefix(raw, "v")
	if idx := strings.IndexByte(raw, '_'); idx >= 0 {
		raw = raw[:idx]
	}
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return gatewayReleaseVersion{}, fmt.Errorf("gateway version %q must be a release SemVer (major.minor.patch)", original)
	}
	values := [3]uint64{}
	for i, part := range parts {
		if part == "" || (len(part) > 1 && part[0] == '0') {
			return gatewayReleaseVersion{}, fmt.Errorf("gateway version %q must be a release SemVer (major.minor.patch)", original)
		}
		value, err := strconv.ParseUint(part, 10, 64)
		if err != nil {
			return gatewayReleaseVersion{}, fmt.Errorf("gateway version %q must be a release SemVer (major.minor.patch)", original)
		}
		values[i] = value
	}
	return gatewayReleaseVersion{major: values[0], minor: values[1], patch: values[2]}, nil
}

func constraintsOverlap(left, right gatewayVersionConstraint) bool {
	leftRange := constraintRange(left)
	rightRange := constraintRange(right)

	lower, lowerInclusive := greaterLowerBound(leftRange.lower, leftRange.lowerInclusive, rightRange.lower, rightRange.lowerInclusive)
	upper, upperInclusive := lesserUpperBound(leftRange.upper, leftRange.upperInclusive, rightRange.upper, rightRange.upperInclusive)
	if lower == nil || upper == nil {
		return true
	}
	comparison := lower.compare(*upper)
	if comparison < 0 {
		return true
	}
	return comparison == 0 && lowerInclusive && upperInclusive
}

type gatewayVersionRange struct {
	lower          *gatewayReleaseVersion
	lowerInclusive bool
	upper          *gatewayReleaseVersion
	upperInclusive bool
}

func constraintRange(constraint gatewayVersionConstraint) gatewayVersionRange {
	version := constraint.version
	switch constraint.operator {
	case ">":
		return gatewayVersionRange{lower: &version}
	case ">=":
		return gatewayVersionRange{lower: &version, lowerInclusive: true}
	case "<":
		return gatewayVersionRange{upper: &version}
	case "<=":
		return gatewayVersionRange{upper: &version, upperInclusive: true}
	default:
		return gatewayVersionRange{lower: &version, lowerInclusive: true, upper: &version, upperInclusive: true}
	}
}

func greaterLowerBound(
	left *gatewayReleaseVersion, leftInclusive bool,
	right *gatewayReleaseVersion, rightInclusive bool,
) (*gatewayReleaseVersion, bool) {
	switch {
	case left == nil:
		return right, rightInclusive
	case right == nil:
		return left, leftInclusive
	}
	switch comparison := left.compare(*right); {
	case comparison > 0:
		return left, leftInclusive
	case comparison < 0:
		return right, rightInclusive
	default:
		return left, leftInclusive && rightInclusive
	}
}

func lesserUpperBound(
	left *gatewayReleaseVersion, leftInclusive bool,
	right *gatewayReleaseVersion, rightInclusive bool,
) (*gatewayReleaseVersion, bool) {
	switch {
	case left == nil:
		return right, rightInclusive
	case right == nil:
		return left, leftInclusive
	}
	switch comparison := left.compare(*right); {
	case comparison < 0:
		return left, leftInclusive
	case comparison > 0:
		return right, rightInclusive
	default:
		return left, leftInclusive && rightInclusive
	}
}
