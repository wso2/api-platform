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

package components

import (
	"errors"
	"fmt"
	"strings"
)

// errorList collects validation errors.
type errorList struct{ errs []error }

func (l *errorList) add(err error) {
	if err != nil {
		l.errs = append(l.errs, err)
	}
}

func (l *errorList) addf(format string, args ...any) {
	l.errs = append(l.errs, fmt.Errorf(format, args...))
}

func (l *errorList) err() error {
	if len(l.errs) == 0 {
		return nil
	}
	return errors.Join(l.errs...)
}

// Validate checks a definition's local invariants.
func (d *Definition) Validate() error {
	if d == nil {
		return fmt.Errorf("component: definition is required")
	}
	var errs errorList

	if strings.TrimSpace(d.Name) == "" {
		return fmt.Errorf("component: name is required")
	}
	modes := 0
	if d.Image.Ref != "" || d.Image.Build != nil {
		modes++
	}
	if d.IsCompose() {
		modes++
	}
	if d.IsExternal() {
		modes++
	}
	if modes != 1 {
		errs.addf("%s: exactly one of image, compose, or external must be configured", d)
	}
	if !d.IsCompose() && !d.IsExternal() && d.Image.Ref == "" && d.Image.Build == nil {
		errs.addf("%s: image must have either a ref or a build", d)
	}
	if strings.TrimSpace(d.Alias) == "" {
		errs.addf("%s: alias is required", d)
	}

	if d.IsExternal() {
		errs.add(d.validateExternal())
	} else {
		errs.add(d.validateEndpoints())
	}
	errs.add(d.validateHealth())
	errs.add(d.validateConfig())
	errs.add(d.validateDB())
	errs.add(d.validateFiles())
	errs.add(d.validateCompose())

	return errs.err()
}

func (d *Definition) validateExternal() error {
	if d.External == nil {
		return nil
	}
	var errs errorList
	if d.Image.Ref != "" || d.Image.Build != nil {
		errs.addf("%s: an external component must not declare an image", d)
	}
	if d.Compose != nil {
		errs.addf("%s: an external component must not declare compose", d)
	}
	if len(d.Endpoints) > 0 {
		errs.addf("%s: an external component must use external endpoints, not container endpoints", d)
	}
	if len(d.External.Endpoints) == 0 {
		errs.addf("%s: external spec declares no endpoints", d)
	}
	if d.External.Resolve == nil {
		errs.addf("%s: external spec has no resolver", d)
	}
	seen := map[string]bool{}
	for i, endpoint := range d.External.Endpoints {
		name := endpoint.Name
		if strings.TrimSpace(name) == "" {
			errs.addf("%s: external endpoint #%d has no name", d, i)
			continue
		}
		if seen[name] {
			errs.addf("%s: duplicate external endpoint name %q", d, name)
		}
		seen[name] = true
	}
	seenParameter := map[string]bool{}
	for _, parameter := range d.External.RequiredParameters {
		if strings.TrimSpace(parameter) == "" {
			errs.addf("%s: external required parameter has no name", d)
		} else if seenParameter[parameter] {
			errs.addf("%s: duplicate external required parameter %q", d, parameter)
		}
		seenParameter[parameter] = true
	}
	if d.Config != nil {
		errs.addf("%s: an external component must not declare config", d)
	}
	if d.DB != nil {
		errs.addf("%s: an external component must not declare a database", d)
	}
	if len(d.Files) > 0 {
		errs.addf("%s: an external component must not declare file mounts", d)
	}
	if len(d.Cmd) > 0 {
		errs.addf("%s: an external component must not declare a command", d)
	}
	if d.Health != nil {
		errs.addf("%s: external components must not declare container health; probe them in product steps", d)
	}
	return errs.err()
}

func (d *Definition) validateEndpoints() error {
	var errs errorList
	seenName := map[string]bool{}
	seenPort := map[int]string{}

	for i, e := range d.Endpoints {
		label := fmt.Sprintf("%q", e.Name)
		if strings.TrimSpace(e.Name) == "" {
			label = fmt.Sprintf("#%d", i)
			errs.addf("%s: endpoint %s has no name", d, label)
		} else {
			if seenName[e.Name] {
				errs.addf("%s: duplicate endpoint name %q", d, e.Name)
			}
			seenName[e.Name] = true
		}

		if e.Port <= 0 || e.Port > 65535 {
			errs.addf("%s: endpoint %s has invalid port %d", d, label, e.Port)
		} else {
			if other, dup := seenPort[e.Port]; dup {
				errs.addf("%s: endpoints %s and %s both use port %d", d, other, label, e.Port)
			}
			seenPort[e.Port] = label
		}

		if strings.TrimSpace(e.Scheme) == "" {
			errs.addf("%s: endpoint %s has no scheme", d, label)
		}
		if e.PathPrefix != "" && !strings.HasPrefix(e.PathPrefix, "/") {
			errs.addf("%s: endpoint %s pathPrefix %q must start with /", d, label, e.PathPrefix)
		}
	}
	return errs.err()
}

func (d *Definition) validateHealth() error {
	var errs errorList
	validate := func(h *HealthCheck, profile string) {
		if h == nil {
			return
		}
		checkLabel := "health check"
		fieldLabel := "health"
		if profile != "" {
			checkLabel = profile
			fieldLabel = profile
		}

		if h.Endpoint == "" {
			errs.addf("%s: %s has no endpoint", d, checkLabel)
		} else if _, ok := d.Endpoint(h.Endpoint); !ok {
			errs.addf("%s: %s references unknown endpoint %q", d, checkLabel, h.Endpoint)
		}
		if h.Path != "" && !strings.HasPrefix(h.Path, "/") {
			errs.addf("%s: %s path %q must start with /", d, fieldLabel, h.Path)
		}
		if h.ExpectStatus < 100 || h.ExpectStatus > 599 {
			errs.addf("%s: %s expectStatus %d is not a valid HTTP status", d, fieldLabel, h.ExpectStatus)
		}
		if h.Timeout <= 0 {
			errs.addf("%s: %s timeout must be positive", d, fieldLabel)
		}
		if h.Interval <= 0 {
			errs.addf("%s: %s interval must be positive", d, fieldLabel)
		}
		if h.Timeout > 0 && h.Interval > 0 && h.Interval > h.Timeout {
			errs.addf("%s: %s interval (%s) exceeds its timeout (%s), so it would probe at most once",
				d, fieldLabel, h.Interval, h.Timeout)
		}
	}

	validate(d.Health, "")
	for version, health := range d.VersionedHealth {
		if strings.TrimSpace(version) == "" {
			errs.addf("%s: has an empty versioned health profile key", d)
		}
		validate(&health, fmt.Sprintf("health profile %q", version))
	}
	return errs.err()
}

func (d *Definition) validateConfig() error {
	if d.Config == nil {
		return nil
	}
	var errs errorList
	c := d.Config

	if c.BaseConfigPath == "" {
		errs.addf("%s: config has no baseConfigPath", d)
	}
	if c.ContainerPath == "" {
		errs.addf("%s: config has no containerPath", d)
	} else if !strings.HasPrefix(c.ContainerPath, "/") {
		errs.addf("%s: config containerPath %q must be absolute", d, c.ContainerPath)
	}
	if c.Format != TOML {
		errs.addf("%s: unsupported config format %q", d, c.Format)
	}
	for version, profile := range c.Versioned {
		if strings.TrimSpace(version) == "" {
			errs.addf("%s: config has an empty versioned profile key", d)
		}
		if strings.TrimSpace(profile.BaseConfigPath) == "" {
			errs.addf("%s: config profile %q has no baseConfigPath", d, version)
		}
	}
	return errs.err()
}

// validateDB checks database contract invariants.
func (d *Definition) validateDB() error {
	if d.DB == nil {
		return nil
	}
	var errs errorList
	c := d.DB

	if len(c.Supported) == 0 {
		errs.addf("%s: has a db contract but supports no engine", d)
	}
	seen := map[DBType]bool{}
	for _, t := range c.Supported {
		if !t.Valid() {
			errs.addf("%s: unknown supported engine %q", d, t)
		}
		if seen[t] {
			errs.addf("%s: engine %q listed twice in supported", d, t)
		}
		seen[t] = true
	}

	if c.Env == nil && c.Owns() {
		errs.addf("%s: owns a store but has no Env mapping, so the DSN would never reach it", d)
	}

	for t := range c.Schema {
		if !t.Valid() {
			errs.addf("%s: schema declared for unknown engine %q", d, t)
			continue
		}
		if !seen[t] {
			errs.addf("%s: schema declared for engine %q, which is not in supported", d, t)
		}
	}
	for _, t := range c.SelfMigrates {
		if !t.Valid() {
			errs.addf("%s: selfMigrates unknown engine %q", d, t)
			continue
		}
		if !seen[t] {
			errs.addf("%s: selfMigrates engine %q, which is not in supported", d, t)
		}
		if _, dup := c.Schema[t]; dup {
			errs.addf("%s: engine %q is in both schema and selfMigrates", d, t)
		}
	}

	if c.SharesStoreWith != "" {
		if len(c.Schema) > 0 {
			errs.addf("%s: shares %q's store but also declares its own schema", d, c.SharesStoreWith)
		}
		if len(c.SelfMigrates) > 0 {
			errs.addf("%s: shares %q's store but also claims to self-migrate", d, c.SharesStoreWith)
		}
	}

	return errs.err()
}

func (d *Definition) validateFiles() error {
	var errs errorList
	seen := map[string]bool{}
	for _, f := range d.Files {
		if f.HostPath == "" {
			errs.addf("%s: file mount has no hostPath", d)
		}
		if f.ContainerPath == "" {
			errs.addf("%s: file mount has no containerPath", d)
			continue
		}
		if !strings.HasPrefix(f.ContainerPath, "/") {
			errs.addf("%s: file mount containerPath %q must be absolute", d, f.ContainerPath)
		}
		if seen[f.ContainerPath] {
			errs.addf("%s: two file mounts target %q", d, f.ContainerPath)
		}
		seen[f.ContainerPath] = true
	}
	return errs.err()
}

// ResolveDBType selects a supported database engine.
func (d *Definition) ResolveDBType(explicit, blockDefault DBType) (DBType, error) {
	if d.DB == nil {
		if explicit != "" {
			return "", fmt.Errorf("%s: has no storage, so db: %q cannot be set on it", d, explicit)
		}
		return "", nil
	}

	chosen := explicit
	if chosen == "" {
		chosen = blockDefault
	}
	if chosen == "" {
		return "", fmt.Errorf("%s: needs a db type and neither the component nor its block specifies one", d)
	}
	if !chosen.Valid() {
		return "", fmt.Errorf("%s: unknown db type %q", d, chosen)
	}
	if !d.DB.Supports(chosen) {
		return "", fmt.Errorf("%s: does not support db %q (supported: %s)", d, chosen, formatDBTypes(d.DB.Supported))
	}
	return chosen, nil
}

func formatDBTypes(types []DBType) string {
	out := make([]string, 0, len(types))
	for _, t := range types {
		out = append(out, string(t))
	}
	return strings.Join(out, ", ")
}
