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

package vault

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// ExternalRefPrefix starts every reference to an entry in a configured
// external resolver:
//
//	kv://<resolver>/<path>[?version=<n>][#<field>]
//
// A secret handle stored in the gateway database cannot contain '/', so a
// reference with this prefix never collides with an existing local handle.
const ExternalRefPrefix = "kv://"

// DefaultField is the field read from a KV entry when the reference names none.
const DefaultField = "value"

// resolverNamePattern restricts resolver names to lower-case DNS labels.
var resolverNamePattern = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]{0,61}[a-z0-9])?$`)

// Ref identifies one credential. Exactly one of Handle and Resolver is set.
type Ref struct {
	// Handle names a secret in the gateway's local encrypted store.
	Handle string

	// Resolver names a configured external resolver, such as a shared OpenBao.
	Resolver string
	// Path is the entry path relative to the resolver's KV mount.
	Path string
	// Field is the key read inside the entry.
	Field string
	// Version pins a KV version. Zero reads the latest version.
	Version int
}

// IsExternal reports whether r points at a configured external resolver.
func (r Ref) IsExternal() bool { return r.Resolver != "" }

// String returns the reference in its canonical form. It never contains a
// credential value, so it is safe to log.
func (r Ref) String() string {
	if !r.IsExternal() {
		return r.Handle
	}
	var b strings.Builder
	b.WriteString(ExternalRefPrefix)
	b.WriteString(r.Resolver)
	b.WriteByte('/')
	b.WriteString(r.Path)
	if r.Version > 0 {
		b.WriteString("?version=")
		b.WriteString(strconv.Itoa(r.Version))
	}
	b.WriteByte('#')
	b.WriteString(r.Field)
	return b.String()
}

// IsExternalRef reports whether s uses the external reference form. Callers
// that sync local handles from a control plane use it to leave external
// references to the vault.
func IsExternalRef(s string) bool {
	return strings.HasPrefix(s, ExternalRefPrefix)
}

// ParseRef parses the argument of a {{ secret "..." }} placeholder. A value
// without the external prefix is a local handle and is returned unchanged.
func ParseRef(s string) (Ref, error) {
	if !IsExternalRef(s) {
		if strings.TrimSpace(s) == "" {
			return Ref{}, fmt.Errorf("secret reference is empty")
		}
		return Ref{Handle: s}, nil
	}

	rest := strings.TrimPrefix(s, ExternalRefPrefix)

	field := DefaultField
	if i := strings.IndexByte(rest, '#'); i >= 0 {
		field = rest[i+1:]
		rest = rest[:i]
		if field == "" {
			return Ref{}, fmt.Errorf("secret reference %q has an empty field after '#'", s)
		}
		if strings.ContainsAny(field, "/?#") || hasControlChar(field) {
			return Ref{}, fmt.Errorf("secret reference %q has an invalid field", s)
		}
	}

	version := 0
	if i := strings.IndexByte(rest, '?'); i >= 0 {
		query := rest[i+1:]
		rest = rest[:i]
		v, ok := strings.CutPrefix(query, "version=")
		if !ok || strings.ContainsAny(v, "&=") {
			return Ref{}, fmt.Errorf("secret reference %q supports only the 'version' query parameter", s)
		}
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			return Ref{}, fmt.Errorf("secret reference %q has an invalid version", s)
		}
		version = n
	}

	resolver, path, ok := strings.Cut(rest, "/")
	if !ok || path == "" {
		return Ref{}, fmt.Errorf("secret reference %q must be kv://<resolver>/<path>", s)
	}
	if !resolverNamePattern.MatchString(resolver) {
		return Ref{}, fmt.Errorf("secret reference %q has an invalid resolver name", s)
	}
	if err := ValidatePath(path); err != nil {
		return Ref{}, fmt.Errorf("secret reference %q: %w", s, err)
	}

	return Ref{Resolver: resolver, Path: path, Field: field, Version: version}, nil
}

// ValidResolverName reports whether name can be used as a resolver name.
func ValidResolverName(name string) bool {
	return resolverNamePattern.MatchString(name)
}

// ValidatePath rejects KV paths that could address another API path: empty,
// "." or ".." segments, and query, fragment, escape or control characters.
func ValidatePath(path string) error {
	for _, seg := range strings.Split(path, "/") {
		if seg == "" || seg == "." || seg == ".." || strings.ContainsAny(seg, "?#%\\") || hasControlChar(seg) {
			return fmt.Errorf("path %q is not a valid KV path", path)
		}
	}
	return nil
}

func hasControlChar(s string) bool {
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			return true
		}
	}
	return false
}
