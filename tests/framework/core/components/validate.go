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
	if !d.IsCompose() && d.Image.Ref == "" && d.Image.Build == nil {
		errs.addf("%s: image must have either a ref or a build", d)
	}
	if strings.TrimSpace(d.Alias) == "" {
		errs.addf("%s: alias is required", d)
	}

	errs.add(d.validateEndpoints())
	errs.add(d.validateHealth())
	errs.add(d.validateConfig())
	errs.add(d.validateDB())
	errs.add(d.validateFiles())
	errs.add(d.validateCompose())

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
	if d.Health == nil {
		return nil
	}
	var errs errorList
	h := d.Health

	if h.Endpoint == "" {
		errs.addf("%s: health check has no endpoint", d)
	} else if _, ok := d.Endpoint(h.Endpoint); !ok {
		errs.addf("%s: health check references unknown endpoint %q", d, h.Endpoint)
	}
	if h.Path != "" && !strings.HasPrefix(h.Path, "/") {
		errs.addf("%s: health path %q must start with /", d, h.Path)
	}
	if h.ExpectStatus < 100 || h.ExpectStatus > 599 {
		errs.addf("%s: health expectStatus %d is not a valid HTTP status", d, h.ExpectStatus)
	}
	if h.Timeout <= 0 {
		errs.addf("%s: health timeout must be positive", d)
	}
	if h.Interval <= 0 {
		errs.addf("%s: health interval must be positive", d)
	}
	if h.Timeout > 0 && h.Interval > 0 && h.Interval > h.Timeout {
		errs.addf("%s: health interval (%s) exceeds its timeout (%s), so it would probe at most once",
			d, h.Interval, h.Timeout)
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
