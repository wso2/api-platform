/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com) All Rights Reserved.
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

package policyv1alpha2

import (
	"regexp"
	"strconv"
	"strings"
	"sync"
)

// Global Backends are gateway-wide backends (kind: Backend). The gateway controller turns
// each revision of one into an Envoy cluster named BackendClusterPrefix+name+"_"+revision and
// publishes a lazy resource of type LazyResourceTypeBackend describing the revision whose
// cluster is in service, so policies and the kernel can resolve a name per request without the
// API naming the Backend in its configuration. A record and the cluster it names always come
// from the same revision, so a request never pairs one revision's base path with another's
// endpoints.

const (
	// LazyResourceTypeBackend is the lazy resource type the gateway controller publishes for
	// every global Backend.
	LazyResourceTypeBackend = "Backend"

	// BackendClusterPrefix prefixes the Envoy cluster of every global Backend. It is
	// reserved: no API-owned cluster uses it.
	BackendClusterPrefix = "backend_"

	// Keys of the lazy resource map the controller publishes for a Backend.
	BackendResourceKeyName        = "name"
	BackendResourceKeyClusterName = "clusterName"
	BackendResourceKeyBasePath    = "basePath"
	BackendResourceKeyLabels      = "labels"
	// BackendResourceKeyURLTemplate is set only for a templated Backend: its endpoint as
	// scheme://host[:port], with BackendKeyPlaceholder in the host's first label.
	BackendResourceKeyURLTemplate = "urlTemplate"

	// BackendKeyPlaceholder marks where a templated Backend's endpoint host takes the
	// per-request key.
	BackendKeyPlaceholder = "{key}"

	backendNameMinLength = 3
	backendNameMaxLength = 40
)

var (
	backendNamePattern     = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	backendRevisionPattern = regexp.MustCompile(`^[0-9a-f]{1,64}$`)
	backendKeyPattern      = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)
	hostLabelPattern       = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)
	labelAffixPattern      = regexp.MustCompile(`^[a-z0-9-]*$`)
)

const (
	maxHostLabelLength = 63
	maxHostLength      = 253
)

// IsValidBackendKey reports whether key can fill a templated Backend's BackendKeyPlaceholder:
// one lowercase DNS label (letters, digits and inner hyphens, at most 63 characters). A key can
// never add a label, a port, a path or userinfo to the host.
func IsValidBackendKey(key string) bool {
	return backendKeyPattern.MatchString(key)
}

// BackendURLTemplate is a templated Backend's endpoint, scheme://host[:port], whose host has
// BackendKeyPlaceholder inside its first label and at least one fixed label after it. The key a
// request carries replaces the placeholder, so every host a templated Backend can reach ends in
// the template's fixed domain.
type BackendURLTemplate struct {
	scheme string
	prefix string // first-label text before the placeholder
	suffix string // first-label text after the placeholder
	domain string // the fixed labels after the first, with the leading "."
	port   string // explicit port, or empty
}

// ParseBackendURLTemplate parses a templated endpoint URL. It accepts http and https, a
// lowercase host whose first label holds exactly one BackendKeyPlaceholder between optional
// fixed text, at least one fixed label after it, the last of which contains a letter (so the
// host can never be an IP address), an optional port, and nothing else. The fixed text must
// leave room for a key: a one-character key must keep the first label within 63 characters and
// the host within 253.
func ParseBackendURLTemplate(raw string) (BackendURLTemplate, bool) {
	var t BackendURLTemplate
	rest, ok := cutScheme(raw, &t)
	if !ok || strings.Count(raw, BackendKeyPlaceholder) != 1 ||
		strings.Count(raw, "{") != 1 || strings.Count(raw, "}") != 1 {
		return BackendURLTemplate{}, false
	}
	rest = strings.TrimSuffix(rest, "/")
	if strings.ContainsAny(rest, "/?#@[]") {
		return BackendURLTemplate{}, false
	}
	host := rest
	if i := strings.LastIndexByte(rest, ':'); i >= 0 {
		host, t.port = rest[:i], rest[i+1:]
		if n, err := strconv.Atoi(t.port); err != nil || n < 1 || n > 65535 || strconv.Itoa(n) != t.port {
			return BackendURLTemplate{}, false
		}
	}
	first, domain, ok := strings.Cut(host, ".")
	if !ok {
		return BackendURLTemplate{}, false
	}
	prefix, suffix, ok := strings.Cut(first, BackendKeyPlaceholder)
	if !ok || !labelAffixPattern.MatchString(prefix) || !labelAffixPattern.MatchString(suffix) ||
		strings.HasPrefix(prefix, "-") || strings.HasSuffix(suffix, "-") {
		return BackendURLTemplate{}, false
	}
	labels := strings.Split(domain, ".")
	for _, label := range labels {
		if !hostLabelPattern.MatchString(label) {
			return BackendURLTemplate{}, false
		}
	}
	if !strings.ContainsAny(labels[len(labels)-1], "abcdefghijklmnopqrstuvwxyz") {
		return BackendURLTemplate{}, false
	}
	t.prefix, t.suffix, t.domain = prefix, suffix, "."+domain
	// The shortest key is one character. A template too long even for it can never expand.
	if label := len(prefix) + 1 + len(suffix); label > maxHostLabelLength || label+len(t.domain) > maxHostLength {
		return BackendURLTemplate{}, false
	}
	return t, true
}

func cutScheme(raw string, t *BackendURLTemplate) (string, bool) {
	for _, scheme := range []string{"https", "http"} {
		if rest, ok := strings.CutPrefix(raw, scheme+"://"); ok {
			t.scheme = scheme
			return rest, true
		}
	}
	return "", false
}

// Scheme is the template's scheme, http or https.
func (t BackendURLTemplate) Scheme() string { return t.scheme }

// String is the template as published: scheme://host[:port].
func (t BackendURLTemplate) String() string {
	return t.scheme + "://" + t.authority(BackendKeyPlaceholder)
}

// Expand fills the placeholder with key. It returns the endpoint URL (scheme://authority), the
// authority (host, or host:port when the template names a port) and whether key is a valid
// key whose expanded first label and host stay within DNS limits.
func (t BackendURLTemplate) Expand(key string) (url, authority string, ok bool) {
	if t.scheme == "" || !IsValidBackendKey(key) {
		return "", "", false
	}
	label := t.prefix + key + t.suffix
	host := label + t.domain
	if len(label) > maxHostLabelLength || len(host) > maxHostLength {
		return "", "", false
	}
	authority = t.authority(key)
	return t.scheme + "://" + authority, authority, true
}

func (t BackendURLTemplate) authority(key string) string {
	authority := t.prefix + key + t.suffix + t.domain
	if t.port != "" {
		authority += ":" + t.port
	}
	return authority
}

// IsValidBackendName reports whether name follows the Backend name rule: lowercase letters,
// digits and single hyphens between them, 3 to 40 characters. A value that fails it can never
// name a Backend, so callers can reject it before any lookup.
func IsValidBackendName(name string) bool {
	return len(name) >= backendNameMinLength && len(name) <= backendNameMaxLength &&
		backendNamePattern.MatchString(name)
}

// BackendInfo is what a policy or the kernel needs to know about one global Backend.
type BackendInfo struct {
	// Name is the Backend's metadata.name.
	Name string
	// ClusterName is the Envoy cluster serving the revision the record describes:
	// BackendClusterPrefix+Name, followed by "_" and a revision when the record names one.
	ClusterName string
	// BasePath is prefixed to the request path after the API context is stripped.
	BasePath string
	// Labels are the Backend's metadata.labels. Shared by every lookup: do not modify.
	Labels map[string]string
	// URLTemplate is set for a templated Backend, whose endpoint host takes a per-request key
	// (UpstreamRequestHeaderModifications.BackendKey). Templated reports whether it is set.
	URLTemplate BackendURLTemplate
}

// Templated reports whether the Backend's endpoint host takes a per-request key.
func (e BackendInfo) Templated() bool {
	return e.URLTemplate.scheme != ""
}

// MatchesLabels reports whether the Backend carries every key/value pair in matchLabels.
// An empty or nil matchLabels matches every Backend, as a Kubernetes label selector does.
func (e BackendInfo) MatchesLabels(matchLabels map[string]string) bool {
	for key, value := range matchLabels {
		if got, ok := e.Labels[key]; !ok || got != value {
			return false
		}
	}
	return true
}

// LookupBackend resolves a global Backend by name from the shared lazy resource store.
func LookupBackend(name string) (BackendInfo, bool) {
	return GetLazyResourceStoreInstance().LookupBackend(name)
}

// backendIndex caches parsed Backend records for one store version, so a per-request
// lookup costs a map read instead of re-parsing the published resource.
type backendIndex struct {
	mu      sync.RWMutex
	built   bool
	version uint64
	byName  map[string]BackendInfo
}

// LookupBackend resolves a global Backend by name. The parsed records are cached and rebuilt
// when the store's version changes.
func (lrs *LazyResourceStore) LookupBackend(name string) (BackendInfo, bool) {
	current := lrs.Version()
	idx := &lrs.backends

	idx.mu.RLock()
	if idx.built && idx.version == current {
		info, ok := idx.byName[name]
		idx.mu.RUnlock()
		return info, ok
	}
	idx.mu.RUnlock()

	idx.mu.Lock()
	defer idx.mu.Unlock()
	if !idx.built || idx.version != current {
		resources, version := lrs.snapshotByType(LazyResourceTypeBackend)
		byName := make(map[string]BackendInfo, len(resources))
		for id, resource := range resources {
			if info, ok := parseBackendResource(id, resource); ok {
				byName[info.Name] = info
			}
		}
		idx.byName, idx.version, idx.built = byName, version, true
	}
	info, ok := idx.byName[name]
	return info, ok
}

// parseBackendResource turns a published Backend lazy resource into a BackendInfo. A record
// whose id is not a valid Backend name, whose clusterName is not a cluster of that Backend, or
// whose urlTemplate does not parse, is skipped, so the cluster a record names can never be
// anything other than one of that Backend's own clusters, and a templated Backend is never
// mistaken for a plain one. A record without a clusterName names BackendClusterPrefix+id.
func parseBackendResource(id string, resource *LazyResource) (BackendInfo, bool) {
	if resource == nil || !IsValidBackendName(id) {
		return BackendInfo{}, false
	}
	info := BackendInfo{Name: id, ClusterName: BackendClusterPrefix + id}
	if cluster, ok := resource.Resource[BackendResourceKeyClusterName].(string); ok && cluster != "" {
		if !isClusterOfBackend(id, cluster) {
			return BackendInfo{}, false
		}
		info.ClusterName = cluster
	}
	if basePath, ok := resource.Resource[BackendResourceKeyBasePath].(string); ok {
		info.BasePath = basePath
	}
	if raw, present := resource.Resource[BackendResourceKeyURLTemplate]; present {
		template, ok := raw.(string)
		if !ok {
			return BackendInfo{}, false
		}
		if info.URLTemplate, ok = ParseBackendURLTemplate(template); !ok {
			return BackendInfo{}, false
		}
	}
	switch labels := resource.Resource[BackendResourceKeyLabels].(type) {
	case map[string]string:
		info.Labels = make(map[string]string, len(labels))
		for key, value := range labels {
			info.Labels[key] = value
		}
	case map[string]interface{}:
		info.Labels = make(map[string]string, len(labels))
		for key, value := range labels {
			if s, ok := value.(string); ok {
				info.Labels[key] = s
			}
		}
	}
	return info, true
}

// isClusterOfBackend reports whether cluster is one of the named Backend's clusters:
// BackendClusterPrefix+name, optionally followed by "_" and a lowercase hex revision. Backend
// names cannot contain "_", so another Backend's cluster never matches.
func isClusterOfBackend(name, cluster string) bool {
	base := BackendClusterPrefix + name
	if cluster == base {
		return true
	}
	revision, ok := strings.CutPrefix(cluster, base+"_")
	return ok && backendRevisionPattern.MatchString(revision)
}
