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
	"context"
	"fmt"
	"strings"
	"time"
)

// DBType is a database engine a component can be backed by.
type DBType string

const (
	// SQLite is embedded in the component container.
	SQLite DBType = "sqlite"
	// Postgres runs as a separate database container.
	Postgres DBType = "postgres"
	// SQLServer runs as a separate database container.
	SQLServer DBType = "sqlserver"
)

// Embedded reports whether this engine runs inside the component's own container
// rather than as a container of its own.
func (d DBType) Embedded() bool { return d == SQLite }

// Valid reports whether d is a known engine.
func (d DBType) Valid() bool {
	switch d {
	case SQLite, Postgres, SQLServer:
		return true
	default:
		return false
	}
}

// DSN contains the resolved coordinates of a logical runtime.
type DSN struct {
	Type DBType

	// Host and Port identify the database on the component network.
	Host string
	Port int

	Database string
	User     string
	Password string
	SSLMode  string

	// FilePath is set only for embedded engines, where there is no host/port.
	FilePath string
}

// DBContract declares a component's database requirements.
type DBContract struct {
	// Supported lists the database engines supported by the component.
	Supported []DBType

	// Schema maps database engines to the ordered DDL files applied by the framework.
	Schema map[DBType][]string

	// SelfMigrates lists engines whose schema is applied by the component.
	SelfMigrates []DBType

	// SharesStoreWith names the component that owns this component's runtime.
	SharesStoreWith string

	// Env maps a resolved DSN to component environment variables.
	Env func(DSN) map[string]string
}

// Owns reports whether this contract provisions its own store rather than borrowing
// another component's.
func (c *DBContract) Owns() bool { return c != nil && c.SharesStoreWith == "" }

// Supports reports whether t is an engine this component can run against.
func (c *DBContract) Supports(t DBType) bool {
	if c == nil {
		return false
	}
	for _, s := range c.Supported {
		if s == t {
			return true
		}
	}
	return false
}

// migratesItself reports whether the component applies its own schema for t.
func (c *DBContract) migratesItself(t DBType) bool {
	if c == nil {
		return false
	}
	for _, s := range c.SelfMigrates {
		if s == t {
			return true
		}
	}
	return false
}

// SchemaFor returns the DDL the framework must apply for t, and whether it must apply
// any at all. A component that migrates itself, or shares another's store, returns
// false — provisioning it anyway would double-apply the schema.
func (c *DBContract) SchemaFor(t DBType) ([]string, bool) {
	if c == nil || !c.Owns() || c.migratesItself(t) {
		return nil, false
	}
	ddl, ok := c.Schema[t]
	if !ok || len(ddl) == 0 {
		return nil, false
	}
	return ddl, true
}

// Endpoint describes an addressable port on a component.
type Endpoint struct {
	// Name identifies the endpoint.
	Name string

	// Port is the in-container port.
	Port int

	// Scheme is "http", "https", "grpc", "tcp", …
	Scheme string

	// PathPrefix is prepended to paths built against this endpoint, for a component
	// that serves its API under a fixed prefix.
	PathPrefix string

	// AwaitListening includes the port in container-level readiness checks.
	AwaitListening bool

	// Service names the Compose service that provides this endpoint. Empty uses the
	// primary service.
	Service string
}

// HealthCheck describes an application-level readiness probe.
type HealthCheck struct {
	// Endpoint names the Endpoint to probe.
	Endpoint string

	// Service names the Compose service to probe. Empty uses the primary service.
	Service string
	// Path is appended to the endpoint's base URL.
	Path string
	// ExpectStatus is the status that means ready. Usually 200.
	ExpectStatus int
	// Timeout bounds the whole poll.
	Timeout time.Duration
	// Interval is the gap between probes.
	Interval time.Duration
}

// ImageRef identifies a container image and optional architecture-specific images.
type ImageRef struct {
	// Ref is the default image, tag included.
	Ref string
	// ByArch overrides Ref for a GOARCH value ("arm64", "amd64").
	ByArch map[string]string
	// Build, when set, means the image is built from the repo rather than pulled.
	// The path is relative to the repository root.
	Build *ImageBuild
}

// WithVersion returns a copy with the image tag replaced by version.
func (i ImageRef) WithVersion(version string) ImageRef {
	version = strings.TrimSpace(version)
	if version == "" {
		return i
	}
	i.Ref = imageWithVersion(i.Ref, version)
	if len(i.ByArch) > 0 {
		byArch := i.ByArch
		i.ByArch = make(map[string]string, len(i.ByArch))
		for arch, ref := range byArch {
			i.ByArch[arch] = imageWithVersion(ref, version)
		}
	}
	return i
}

func imageWithVersion(ref, version string) string {
	if ref == "" {
		return ref
	}
	lastSlash := strings.LastIndexByte(ref, '/')
	lastColon := strings.LastIndexByte(ref, ':')
	if lastColon <= lastSlash {
		return ref + ":" + version
	}
	return ref[:lastColon+1] + version
}

// ImageBuild describes building an image from the repository.
type ImageBuild struct {
	Context    string
	Dockerfile string
	Args       map[string]string
}

// Resolve returns the image to use on arch.
func (i ImageRef) Resolve(arch string) string {
	if alt, ok := i.ByArch[arch]; ok && alt != "" {
		return alt
	}
	return i.Ref
}

// ResourceLimits specifies per-container CPU and memory limits.
type ResourceLimits struct {
	CPUs     float64
	MemoryMB int64
}

// ConfigFormat is the syntax of a component's configuration file.
type ConfigFormat string

// TOML identifies TOML component configuration.
const TOML ConfigFormat = "toml"

// ConfigInjection describes how a component configuration is assembled and delivered.
type ConfigInjection struct {
	// BaseConfigPath is the product's shipped config, relative to the repo root.
	BaseConfigPath string
	// SharedOverlayPath is the small overlay every block gets, relative to the repo
	// root. Optional.
	SharedOverlayPath string
	// ExtraOverlays are optional overlays merged after the shared overlay and before the
	// block-specific overlay.
	ExtraOverlays []string
	// ContainerPath is where the merged file lands in the container.
	ContainerPath string
	// Format is the merge syntax.
	Format ConfigFormat
}

// FileMount describes a file copied into a container before startup.
type FileMount struct {
	// HostPath is relative to the repository root.
	HostPath string
	// ContainerPath is the absolute destination.
	ContainerPath string
	// Mode is the file mode to set on the copy.
	Mode int64
}

// Definition describes a component and its lifecycle contracts.
type Definition struct {
	// Name is the identifier a suite file references. Unique within a registry.
	Name string

	// Image is what to run, for a single-container component.
	Image ImageRef

	// Compose backs this component with a multi-container stack instead of a single
	// image, while still presenting it as ONE component to a suite file. Mutually
	// exclusive with Image.
	Compose *ComposeSpec

	// Alias is the component's DNS name on its network.
	Alias string

	// AliasIsFixed prevents the alias from being suffixed for replicas.
	AliasIsFixed bool

	// Endpoints are the component's addressable ports.
	Endpoints []Endpoint

	// Health is the application-level readiness gate. Nil disables this gate.
	Health *HealthCheck

	// Config describes configuration assembly, if the component has any.
	Config *ConfigInjection

	// DB is the storage contract, if the component has storage.
	DB *DBContract

	// Wiring decodes and validates component-specific block configuration.
	Wiring WiringSpec

	// Env is static environment applied to every instance, before DSN and wiring.
	Env map[string]string

	// Files are copied in before start.
	Files []FileMount

	// Cmd overrides the image's command.
	Cmd []string

	// Provisions returns environment values for dependent components after this component
	// becomes ready.
	Provisions func(ctx context.Context, inst *Instance) (map[string]string, error)

	// DependsOn lists components that must be ready before this one starts.
	DependsOn []string

	// Limits caps this container's host resources.
	Limits ResourceLimits

	// Shared starts one instance for all blocks that declare the component.
	Shared bool
}

// WithImageVersion returns a copy whose image references use version.
func (d *Definition) WithImageVersion(version string) *Definition {
	if d == nil || strings.TrimSpace(version) == "" {
		return d
	}
	out := *d
	out.Image = d.Image.WithVersion(version)
	if d.Compose != nil {
		compose := *d.Compose
		if d.Compose.Env != nil {
			compose.Env = make(map[string]string, len(d.Compose.Env))
			for key, ref := range d.Compose.Env {
				if strings.HasSuffix(key, "_IMAGE") {
					compose.Env[key] = imageWithVersion(ref, version)
				} else {
					compose.Env[key] = ref
				}
			}
		}
		out.Compose = &compose
	}
	return &out
}

// Endpoint returns the named endpoint.
func (d *Definition) Endpoint(name string) (Endpoint, bool) {
	for _, e := range d.Endpoints {
		if e.Name == name {
			return e, true
		}
	}
	return Endpoint{}, false
}

// String renders the definition for error messages.
func (d *Definition) String() string {
	return fmt.Sprintf("component %q", d.Name)
}
