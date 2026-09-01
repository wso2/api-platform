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
	"fmt"
	"strings"
)

// ComposeSpec describes a component backed by a Compose stack.
type ComposeSpec struct {
	// ComposeFile is the repo-relative compose file. Its services are this component's
	// internals.
	ComposeFile string

	// StagedFiles maps repository-relative source files to their names beside the
	// staged Compose file.
	StagedFiles map[string]string

	// GeneratedFiles maps framework-generated file names to their contents.
	GeneratedFiles map[string][]byte

	// PrimaryService is the compose service whose ports back this component's
	// Endpoints. Everything a test addresses must be on this service.
	PrimaryService string

	// Services lists the services whose logs are collected.
	Services []string

	// Env contains variables passed to Compose for interpolation.
	Env map[string]string

	// CoverageServices lists services and artifact formats collected after shutdown.
	CoverageServices []CoverageService
}

// CoverageService identifies a service that writes coverage artifacts.
type CoverageService struct {
	Name  string
	Types []string
}

// IsCompose reports whether this component is backed by a compose stack rather than a
// single container.
func (d *Definition) IsCompose() bool { return d.Compose != nil }

func (d *Definition) validateCompose() error {
	if d.Compose == nil {
		return nil
	}
	var errs errorList
	c := d.Compose

	if strings.TrimSpace(c.ComposeFile) == "" {
		errs.addf("%s: compose spec has no compose file", d)
	}
	if strings.TrimSpace(c.PrimaryService) == "" {
		errs.addf("%s: compose spec has no primary service", d)
	}
	if len(c.Services) == 0 {
		errs.addf("%s: compose spec lists no services, so no logs would be captured", d)
	} else if c.PrimaryService != "" && !containsStr(c.Services, c.PrimaryService) {
		errs.addf("%s: primary service %q is not in the services list %v", d, c.PrimaryService, c.Services)
	}

	// Compose components obtain their images from the Compose file.
	if d.Image.Ref != "" || d.Image.Build != nil {
		errs.addf("%s: a compose-backed component must not also declare an image", d)
	}

	// Compose publishes ports; endpoints describe how tests address the component.
	if len(d.Endpoints) == 0 {
		errs.addf("%s: a compose-backed component still needs endpoints for addressing", d)
	}

	for name, rel := range c.StagedFiles {
		if strings.TrimSpace(name) == "" {
			errs.addf("%s: a staged file has no name", d)
		}
		if strings.TrimSpace(rel) == "" {
			errs.addf("%s: staged file %q has no source path", d, name)
		}
	}
	seenCoverage := make(map[string]bool, len(c.CoverageServices))
	for _, service := range c.CoverageServices {
		if strings.TrimSpace(service.Name) == "" {
			errs.addf("%s: coverage service has no name", d)
			continue
		}
		if seenCoverage[service.Name] {
			errs.addf("%s: coverage service %q is duplicated", d, service.Name)
		}
		seenCoverage[service.Name] = true
		if !containsStr(c.Services, service.Name) {
			errs.addf("%s: coverage service %q is not in the services list", d, service.Name)
		}
		if len(service.Types) == 0 {
			errs.addf("%s: coverage service %q has no coverage types", d, service.Name)
		}
		for _, coverageType := range service.Types {
			if coverageType != "go" && coverageType != "node-v8" {
				errs.addf("%s: coverage service %q has unsupported type %q", d, service.Name, coverageType)
			}
		}
	}

	return errs.err()
}

func containsStr(haystack []string, needle string) bool {
	for _, h := range haystack {
		if h == needle {
			return true
		}
	}
	return false
}

// StagingName returns the file name the compose file is staged under.
func (c *ComposeSpec) StagingName() string {
	if i := strings.LastIndex(c.ComposeFile, "/"); i >= 0 {
		return c.ComposeFile[i+1:]
	}
	return c.ComposeFile
}

// WithGenerated returns a copy of the spec with generated files added.
func (c *ComposeSpec) WithGenerated(files map[string][]byte) *ComposeSpec {
	out := &ComposeSpec{
		ComposeFile:      c.ComposeFile,
		PrimaryService:   c.PrimaryService,
		Services:         append([]string(nil), c.Services...),
		StagedFiles:      make(map[string]string, len(c.StagedFiles)),
		GeneratedFiles:   make(map[string][]byte, len(files)+len(c.GeneratedFiles)),
		Env:              make(map[string]string, len(c.Env)),
		CoverageServices: append([]CoverageService(nil), c.CoverageServices...),
	}
	for k, v := range c.StagedFiles {
		out.StagedFiles[k] = v
	}
	for k, v := range c.Env {
		out.Env[k] = v
	}
	for k, v := range c.GeneratedFiles {
		out.GeneratedFiles[k] = append([]byte(nil), v...)
	}
	for k, v := range files {
		out.GeneratedFiles[k] = append([]byte(nil), v...)
	}
	return out
}

// EnvFileContent renders environment entries for a Compose env file.
func EnvFileContent(env map[string]string) []byte {
	var b strings.Builder
	keys := sortedKeys(env)
	for _, k := range keys {
		fmt.Fprintf(&b, "%s=%s\n", k, escapeDollars(env[k]))
	}
	return []byte(b.String())
}

// escapeDollars doubles every '$' so compose emits it literally.
func escapeDollars(v string) string {
	return strings.ReplaceAll(v, "$", "$$")
}

func sortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}
