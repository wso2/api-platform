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

// Package valuespolicy checks per-gateway Helm values before the operator
// renders the gateway chart with them.
//
// The values checked here are the overlay the operator builds for one gateway:
// the APIGateway (or Gateway) spec overlay merged with the ConfigMap it
// references. Anyone who can edit that ConfigMap can change these values, and
// the operator applies them with its own, much broader, permissions. The
// operator's base values file is set by whoever installs the operator and is
// not checked.
//
// The policy is a short, explicit list of refusals for the values that would
// let such an edit change what code runs, or with which privileges:
//
//   - image repositories outside the allowed registries;
//   - extraEnv / extraEnvFrom entries that read a Secret;
//   - extraVolumes of any type other than emptyDir, configMap, downwardAPI,
//     or projected configMap/downwardAPI sources (so no secret, hostPath,
//     persistentVolumeClaim, CSI, ...), and extraVolumeMounts with mount
//     propagation;
//   - security contexts that are privileged, allow privilege escalation, run
//     as root, add capabilities, or relax seccomp/AppArmor/SELinux/procMount,
//     and explicit nulls that would erase a hardened default;
//   - hostNetwork, hostPID and hostIPC.
//
// The chart's own Secret-name settings (for example
// gateway.controller.tls.secret.name or controlPlane.token.secretName) are
// part of normal gateway configuration and are not refused.
package valuespolicy

import (
	"fmt"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// DefaultAllowedImageRegistries are the image repository prefixes the gateway
// chart itself uses: its default images, and the registry the chart switches
// to when a subscription pull secret is set.
var DefaultAllowedImageRegistries = []string{
	"ghcr.io/wso2/api-platform/",
	"registry.wso2.com/wso2-api-platform/",
}

// Policy is the values policy configuration.
type Policy struct {
	// Enabled turns the policy on. When false, Check accepts everything.
	Enabled bool
	// AllowedImageRegistries lists the image repository prefixes an overlay may
	// use. An entry matches the repository itself or anything below it at a
	// path boundary ("ghcr.io/org" matches "ghcr.io/org/img", not
	// "ghcr.io/organisation/img"). An empty list refuses every override.
	AllowedImageRegistries []string
}

// Violation is one refused value.
type Violation struct {
	// Path is the dotted path of the offending key, with list indexes, for
	// example gateway.controller.deployment.extraEnv[0].valueFrom.secretKeyRef.
	Path string
	// Reason says why the value is refused.
	Reason string
}

func (v Violation) String() string { return v.Path + ": " + v.Reason }

// ViolationError is returned by Check when the values break the policy.
type ViolationError struct {
	Violations []Violation
}

func (e *ViolationError) Error() string {
	parts := make([]string, 0, len(e.Violations))
	for _, v := range e.Violations {
		parts = append(parts, v.String())
	}
	return "gateway values refused by values policy: " + strings.Join(parts, "; ")
}

// Keys returns the offending key paths.
func (e *ViolationError) Keys() []string {
	keys := make([]string, 0, len(e.Violations))
	for _, v := range e.Violations {
		keys = append(keys, v.Path)
	}
	return keys
}

// Check validates a Helm values overlay (YAML) against the policy. It returns
// nil when the values are allowed or the policy is disabled, a
// *ViolationError when they are refused, and a plain error when the YAML
// cannot be parsed.
func (p Policy) Check(valuesYAML string) error {
	if !p.Enabled || strings.TrimSpace(valuesYAML) == "" {
		return nil
	}
	var root interface{}
	if err := yaml.Unmarshal([]byte(valuesYAML), &root); err != nil {
		return fmt.Errorf("parse gateway values: %w", err)
	}
	c := checker{policy: p}
	c.walk("", "", normalize(root))
	if len(c.violations) == 0 {
		return nil
	}
	sort.SliceStable(c.violations, func(i, j int) bool { return c.violations[i].Path < c.violations[j].Path })
	return &ViolationError{Violations: c.violations}
}

type checker struct {
	policy     Policy
	violations []Violation
}

func (c *checker) refuse(path, reason string) {
	c.violations = append(c.violations, Violation{Path: path, Reason: reason})
}

func join(path, key string) string {
	if path == "" {
		return key
	}
	return path + "." + key
}

func index(path string, i int) string { return fmt.Sprintf("%s[%d]", path, i) }

// walk visits every value. key is the map key that holds v ("" for list items
// and the root); checks keyed on a field name run when that name is seen at
// any depth, so they also cover keys a future chart version adds.
func (c *checker) walk(path, key string, v interface{}) {
	switch key {
	case "hostNetwork", "hostPID", "hostIPC":
		if isTrue(v) {
			c.refuse(path, "host namespaces are not allowed")
		}
		return
	case "image":
		c.checkImage(path, v)
	case "securityContext", "podSecurityContext":
		c.checkSecurityContext(path, v)
		return
	case "extraEnv":
		c.checkEnv(path, v)
		return
	case "extraEnvFrom":
		c.checkEnvFrom(path, v)
		return
	case "extraVolumes":
		c.checkVolumes(path, v)
		return
	case "extraVolumeMounts":
		c.checkVolumeMounts(path, v)
		return
	}

	switch t := v.(type) {
	case map[string]interface{}:
		for k, child := range t {
			c.walk(join(path, k), k, child)
		}
	case []interface{}:
		for i, child := range t {
			c.walk(index(path, i), "", child)
		}
	}
}

// checkImage handles the chart's image blocks ({repository, tag, pullPolicy})
// and a plain image reference string.
func (c *checker) checkImage(path string, v interface{}) {
	switch t := v.(type) {
	case map[string]interface{}:
		if repo, ok := t["repository"].(string); ok && repo != "" && !c.registryAllowed(repo) {
			c.refuse(join(path, "repository"), fmt.Sprintf("image repository %q is not in the allowed registries", repo))
		}
	case string:
		if t != "" && !c.registryAllowed(t) {
			c.refuse(path, fmt.Sprintf("image %q is not in the allowed registries", t))
		}
	}
}

func (c *checker) registryAllowed(repo string) bool {
	repo = strings.TrimSpace(repo)
	for _, allowed := range c.policy.AllowedImageRegistries {
		allowed = strings.TrimSuffix(strings.TrimSpace(allowed), "/")
		if allowed == "" {
			continue
		}
		if repo == allowed || strings.HasPrefix(repo, allowed+"/") {
			return true
		}
		// A plain image reference may carry a tag or digest directly after the
		// allowed repository.
		if strings.HasPrefix(repo, allowed+":") || strings.HasPrefix(repo, allowed+"@") {
			return true
		}
	}
	return false
}

// securityContextNoNull are fields an overlay may not erase with an explicit
// null, since that would drop a hardened value from the operator's base file.
var securityContextNoNull = []string{
	"runAsNonRoot", "runAsUser", "allowPrivilegeEscalation", "privileged",
	"capabilities", "readOnlyRootFilesystem", "seccompProfile",
}

func (c *checker) checkSecurityContext(path string, v interface{}) {
	if v == nil {
		c.refuse(path, "may not be set to null")
		return
	}
	sc, ok := v.(map[string]interface{})
	if !ok {
		return
	}
	for _, f := range securityContextNoNull {
		if val, present := sc[f]; present && val == nil {
			c.refuse(join(path, f), "may not be set to null")
		}
	}
	if isTrue(sc["privileged"]) {
		c.refuse(join(path, "privileged"), "privileged containers are not allowed")
	}
	if isTrue(sc["allowPrivilegeEscalation"]) {
		c.refuse(join(path, "allowPrivilegeEscalation"), "privilege escalation is not allowed")
	}
	if isFalse(sc["runAsNonRoot"]) {
		c.refuse(join(path, "runAsNonRoot"), "must not be false")
	}
	if isZero(sc["runAsUser"]) {
		c.refuse(join(path, "runAsUser"), "running as root (UID 0) is not allowed")
	}
	if procMount, ok := sc["procMount"].(string); ok && procMount != "" && procMount != "Default" {
		c.refuse(join(path, "procMount"), "only the Default procMount is allowed")
	}
	if caps, ok := sc["capabilities"].(map[string]interface{}); ok {
		if add, ok := caps["add"].([]interface{}); ok {
			for i, capability := range add {
				if s, _ := capability.(string); strings.TrimPrefix(strings.ToUpper(s), "CAP_") != "NET_BIND_SERVICE" {
					c.refuse(index(join(path, "capabilities.add"), i), fmt.Sprintf("adding capability %v is not allowed", capability))
				}
			}
		}
	}
	for _, f := range []string{"seccompProfile", "appArmorProfile"} {
		if prof, ok := sc[f].(map[string]interface{}); ok {
			if t, _ := prof["type"].(string); t == "Unconfined" {
				c.refuse(join(path, f+".type"), "Unconfined is not allowed")
			}
		}
	}
	if se, ok := sc["seLinuxOptions"].(map[string]interface{}); ok {
		for _, f := range []string{"type", "user", "role"} {
			if s, _ := se[f].(string); s != "" {
				c.refuse(join(path, "seLinuxOptions."+f), "setting SELinux "+f+" is not allowed")
			}
		}
	}
	if win, ok := sc["windowsOptions"].(map[string]interface{}); ok {
		if isTrue(win["hostProcess"]) {
			c.refuse(join(path, "windowsOptions.hostProcess"), "host process containers are not allowed")
		}
	}
}

// isTrue and isFalse also accept the string spellings a later YAML or JSON
// conversion could turn into a boolean, so quoting a value does not slip it past.
func isTrue(v interface{}) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		switch strings.ToLower(strings.TrimSpace(t)) {
		case "true", "yes", "y", "on":
			return true
		}
	}
	return false
}

func isFalse(v interface{}) bool {
	switch t := v.(type) {
	case bool:
		return !t
	case string:
		switch strings.ToLower(strings.TrimSpace(t)) {
		case "false", "no", "n", "off":
			return true
		}
	}
	return false
}

// normalize turns maps with non-string keys into string-keyed maps so every
// key is visited.
func normalize(v interface{}) interface{} {
	switch t := v.(type) {
	case map[string]interface{}:
		out := make(map[string]interface{}, len(t))
		for k, child := range t {
			out[k] = normalize(child)
		}
		return out
	case map[interface{}]interface{}:
		out := make(map[string]interface{}, len(t))
		for k, child := range t {
			out[fmt.Sprint(k)] = normalize(child)
		}
		return out
	case []interface{}:
		out := make([]interface{}, len(t))
		for i, child := range t {
			out[i] = normalize(child)
		}
		return out
	}
	return v
}

func isZero(v interface{}) bool {
	switch n := v.(type) {
	case int:
		return n == 0
	case int64:
		return n == 0
	case uint64:
		return n == 0
	case float64:
		return n == 0
	case string:
		return strings.TrimSpace(n) == "0"
	}
	return false
}

func (c *checker) checkEnv(path string, v interface{}) {
	list, ok := v.([]interface{})
	if !ok {
		return
	}
	for i, item := range list {
		env, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		vf, ok := env["valueFrom"].(map[string]interface{})
		if !ok {
			continue
		}
		if _, ok := vf["secretKeyRef"]; ok {
			c.refuse(join(index(path, i), "valueFrom.secretKeyRef"), "environment variables may not read Secrets")
		}
	}
}

func (c *checker) checkEnvFrom(path string, v interface{}) {
	list, ok := v.([]interface{})
	if !ok {
		return
	}
	for i, item := range list {
		src, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if _, ok := src["secretRef"]; ok {
			c.refuse(join(index(path, i), "secretRef"), "envFrom may not read Secrets")
		}
	}
}

// allowedVolumeSources are the volume types an overlay may add.
var allowedVolumeSources = map[string]bool{
	"emptyDir":    true,
	"configMap":   true,
	"downwardAPI": true,
	"projected":   true, // sources checked below
}

// allowedProjectedSources are the projected volume sources an overlay may use.
var allowedProjectedSources = map[string]bool{
	"configMap":   true,
	"downwardAPI": true,
}

func (c *checker) checkVolumes(path string, v interface{}) {
	list, ok := v.([]interface{})
	if !ok {
		return
	}
	for i, item := range list {
		vol, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		volPath := index(path, i)
		for k, src := range vol {
			if k == "name" {
				continue
			}
			if !allowedVolumeSources[k] {
				c.refuse(join(volPath, k), fmt.Sprintf("volume type %q is not allowed (allowed: emptyDir, configMap, downwardAPI, projected)", k))
				continue
			}
			if k != "projected" {
				continue
			}
			proj, _ := src.(map[string]interface{})
			sources, _ := proj["sources"].([]interface{})
			for j, s := range sources {
				sm, _ := s.(map[string]interface{})
				for sk := range sm {
					if !allowedProjectedSources[sk] {
						c.refuse(join(index(join(volPath, "projected.sources"), j), sk), fmt.Sprintf("projected source %q is not allowed (allowed: configMap, downwardAPI)", sk))
					}
				}
			}
		}
	}
}

func (c *checker) checkVolumeMounts(path string, v interface{}) {
	list, ok := v.([]interface{})
	if !ok {
		return
	}
	for i, item := range list {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if p, _ := m["mountPropagation"].(string); p != "" && p != "None" {
			c.refuse(join(index(path, i), "mountPropagation"), "mount propagation is not allowed")
		}
	}
}
