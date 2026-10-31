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

// permission_tree_test.go — build-time consistency between the three files that
// define authorization: resources/permission-tree.yaml (nodes, actions and each
// operation's binding under x-operation-permissions), the shipped OpenAPI specs
// (each operation's generated accepted-scope list) and
// resources/legacy-scope-mapping.yaml. It re-derives every accepted list from
// the tree's inheritance rule and fails when a spec, the tree or the legacy map
// disagree, which is what makes "ap:api:manage admits every operation under
// ap:api" a guarantee rather than a convention. scripts/permission_tree.py
// --check performs the same comparison outside Go.

package server

import (
	"os"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/middleware"
)

const (
	realPermissionTreePath = "../../resources/permission-tree.yaml"
	realLegacyScopeMapPath = "../../resources/legacy-scope-mapping.yaml"
)

// loadShippedLegacyScopeMap loads the legacy scope mapping shipped in resources/.
func loadShippedLegacyScopeMap(t *testing.T) middleware.LegacyScopeMap {
	t.Helper()
	m, err := middleware.LoadLegacyScopeMap(realLegacyScopeMapPath)
	if err != nil {
		t.Fatalf("LoadLegacyScopeMap(%q): %v", realLegacyScopeMapPath, err)
	}
	return m
}

// treeNode is one resource of resources/permission-tree.yaml.
type treeNode struct {
	Handle    string     `yaml:"handle"`
	Actions   []string   `yaml:"actions"`
	Legacy    []string   `yaml:"legacy"`
	Resources []treeNode `yaml:"resources"`
}

// permissionNode is a tree node with its position resolved.
type permissionNode struct {
	path    string // "ap:api:rest"
	actions map[string]bool
	parent  *permissionNode
}

func (n *permissionNode) perm(action string) string { return n.path + ":" + action }

// permissionTree is the parsed tree: every node by path, every permission it
// defines, the node each pre-tree resource was absorbed into, and the
// permissions each operation is bound to (x-operation-permissions, keyed by
// operationId or by "METHOD /path" for an operation that has none).
type permissionTree struct {
	nodes    map[string]*permissionNode
	perms    map[string]*permissionNode // "ap:api:rest:read" -> node
	legacy   map[string]*permissionNode // "rest_api:api_key" -> node
	bindings map[string][]string        // "CreateRESTAPI" -> permissions it lists directly
}

func loadPermissionTree(t *testing.T) *permissionTree {
	t.Helper()
	data, err := os.ReadFile(realPermissionTreePath)
	if err != nil {
		t.Fatalf("read %q: %v", realPermissionTreePath, err)
	}
	var doc struct {
		Tree     []treeNode          `yaml:"tree"`
		Bindings map[string][]string `yaml:"x-operation-permissions"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse %q: %v", realPermissionTreePath, err)
	}
	tree := &permissionTree{
		nodes:    map[string]*permissionNode{},
		perms:    map[string]*permissionNode{},
		legacy:   map[string]*permissionNode{},
		bindings: doc.Bindings,
	}
	var walk func(items []treeNode, parent *permissionNode)
	walk = func(items []treeNode, parent *permissionNode) {
		for _, raw := range items {
			path := raw.Handle
			if parent != nil {
				path = parent.path + ":" + raw.Handle
			}
			n := &permissionNode{path: path, actions: map[string]bool{}, parent: parent}
			for _, a := range raw.Actions {
				n.actions[a] = true
				tree.perms[n.perm(a)] = n
			}
			for _, l := range raw.Legacy {
				tree.legacy[l] = n
			}
			tree.nodes[path] = n
			walk(raw.Resources, n)
		}
	}
	walk(doc.Tree, nil)
	if len(tree.bindings) == 0 {
		t.Fatalf("%s declares no x-operation-permissions", realPermissionTreePath)
	}
	return tree
}

// inherit is the inheritance rule of permission-tree.yaml: an operation bound
// to node n with the given action accepts that action's permission on n, :manage
// on n and on every ancestor that defines it, and — for a read — :read on every
// ancestor that defines it. Only read and manage flow down the tree; a parent's
// create, update, delete, ... never reaches a child. Most specific first, the
// order the generator writes.
func (tree *permissionTree) inherit(n *permissionNode, action string) []string {
	var out []string
	for x := n; x != nil; x = x.parent {
		// The bound action on the node itself; on an ancestor only read and
		// manage are inherited.
		if x.actions[action] && (x == n || action == "read" || action == "manage") {
			out = append(out, x.perm(action))
		}
		if action != "manage" && x.actions["manage"] {
			out = append(out, x.perm("manage"))
		}
	}
	return out
}

// bind resolves the node and action an operation is bound to from the
// permissions it lists directly: the deepest tree permission names the node,
// and its non-manage action (if it lists one) the action.
func (tree *permissionTree) bind(t *testing.T, key string, explicit []string) (*permissionNode, string) {
	t.Helper()
	var node *permissionNode
	depth := -1
	var actions []string
	for _, s := range explicit {
		if s == constants.ScopeAPIKeyAllManage {
			continue // the ownership override widens whose keys, not which node
		}
		n, ok := tree.perms[s]
		if !ok {
			t.Fatalf("%s lists %q, which permission-tree.yaml does not define", key, s)
		}
		d := strings.Count(n.path, ":")
		if d < depth {
			continue
		}
		if d > depth {
			node, depth, actions = n, d, nil
		} else if n != node {
			t.Fatalf("%s lists permissions of two nodes at the same depth: %s and %s", key, node.path, n.path)
		}
		if action := s[strings.LastIndex(s, ":")+1:]; action != "manage" {
			actions = append(actions, action)
		}
	}
	if node == nil {
		t.Fatalf("%s lists no permission-tree permission: %v", key, explicit)
	}
	if len(actions) > 1 {
		t.Fatalf("%s binds %s to two actions: %v", key, node.path, actions)
	}
	if len(actions) == 1 {
		return node, actions[0]
	}
	return node, "manage"
}

// accepted is the full list an operation must declare: the inheritance rule
// applied to its binding, then every cross-resource grant and the ownership
// override it lists directly.
func (tree *permissionTree) accepted(t *testing.T, key string, explicit []string) []string {
	t.Helper()
	node, action := tree.bind(t, key, explicit)
	out := tree.inherit(node, action)
	have := map[string]bool{}
	for _, s := range out {
		have[s] = true
	}
	chain := map[string]bool{}
	for x := node; x != nil; x = x.parent {
		chain[x.path] = true
	}
	for _, s := range explicit {
		if have[s] {
			continue
		}
		if s != constants.ScopeAPIKeyAllManage && chain[tree.perms[s].path] {
			t.Fatalf("%s lists %q, which is on the path of its binding %s:%s but not what the inheritance rule grants there", key, s, node.path, action)
		}
		have[s] = true
		out = append(out, s)
	}
	return out
}

// specOperation is one operation of a shipped OpenAPI spec and the scopes its
// security block declares. path carries the spec's server base path, the form
// the scope registry keys on.
type specOperation struct {
	key, method, path string
	scopes            []string
}

// specBasePath is the path of the spec's first server URL ("/api/v0.9"), as
// LoadScopeRegistry prefixes it; empty when the spec declares no server.
func specBasePath(servers []struct {
	URL string `yaml:"url"`
}) string {
	if len(servers) == 0 {
		return ""
	}
	s := servers[0].URL
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
	}
	if i := strings.Index(s, "/"); i >= 0 {
		return strings.TrimRight(s[i:], "/")
	}
	return ""
}

var httpMethods = map[string]bool{"get": true, "put": true, "post": true, "delete": true,
	"patch": true, "head": true, "options": true, "trace": true}

// loadSpecOperations parses the shipped main and plugin specs directly, so the
// comparison below is against what the files say and not only against what the
// registry loader kept.
func loadSpecOperations(t *testing.T) []specOperation {
	t.Helper()
	var ops []specOperation
	for _, specPath := range []string{realSpecPath, pluginSpecPath} {
		data, err := os.ReadFile(specPath)
		if err != nil {
			t.Fatalf("read %q: %v", specPath, err)
		}
		var doc struct {
			Servers []struct {
				URL string `yaml:"url"`
			} `yaml:"servers"`
			Paths map[string]map[string]any `yaml:"paths"`
		}
		if err := yaml.Unmarshal(data, &doc); err != nil {
			t.Fatalf("parse %q: %v", specPath, err)
		}
		basePath := specBasePath(doc.Servers)
		for path, item := range doc.Paths {
			for method, raw := range item {
				if !httpMethods[method] {
					continue
				}
				op, _ := raw.(map[string]any)
				key, _ := op["operationId"].(string)
				if key == "" {
					key = strings.ToUpper(method) + " " + path
				}
				var scopes []string
				if security, ok := op["security"].([]any); ok {
					for _, rawReq := range security {
						req, _ := rawReq.(map[string]any)
						for _, rawScopes := range req {
							list, _ := rawScopes.([]any)
							for _, s := range list {
								scopes = append(scopes, s.(string))
							}
						}
					}
				}
				ops = append(ops, specOperation{key: key, method: strings.ToUpper(method), path: basePath + path, scopes: scopes})
			}
		}
	}
	sort.Slice(ops, func(i, j int) bool { return ops[i].key < ops[j].key })
	return ops
}

func setOf(scopes []string) map[string]bool {
	m := make(map[string]bool, len(scopes))
	for _, s := range scopes {
		m[s] = true
	}
	return m
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// TestEveryOperationAcceptsExactlyItsPermissionTreeBinding is the build-time
// guarantee behind ScopeEnforcer's parent-permission behaviour. The enforcer
// matches an operation's accepted list by exact string equality and never walks
// the tree itself, so "ap:api:manage admits a request bound to
// ap:api:rest:deployment:undeploy" holds only because the generated list names
// every ancestor's manage (and, for a read, read). Each operation's security block must therefore
// be exactly what scripts/permission_tree.py derives from its binding in
// x-operation-permissions: the inheritance rule's permissions plus the
// cross-resource grants and ownership override the binding lists. A pre-tree
// name left in the spec would bypass the legacy mapping; an undefined string
// would be grantable by nobody; a missing ancestor would silently deny a role
// granted at the parent.
func TestEveryOperationAcceptsExactlyItsPermissionTreeBinding(t *testing.T) {
	tree := loadPermissionTree(t)
	registry := loadMergedRegistry(t)
	bound := map[string]bool{}

	for _, op := range loadSpecOperations(t) {
		explicit, ok := tree.bindings[op.key]
		if !ok {
			t.Errorf("%s %s (%s) has no x-operation-permissions entry in %s", op.method, op.path, op.key, realPermissionTreePath)
			continue
		}
		bound[op.key] = true
		want, have := setOf(tree.accepted(t, op.key, explicit)), setOf(op.scopes)
		for _, s := range sortedKeys(want) {
			if !have[s] {
				t.Errorf("%s %s (%s) does not accept %q, which its binding %v grants (accepts %v)", op.method, op.path, op.key, s, explicit, op.scopes)
			}
		}
		for _, s := range sortedKeys(have) {
			if !want[s] {
				t.Errorf("%s %s (%s) accepts %q, which its binding %v does not grant", op.method, op.path, op.key, s, explicit)
			}
		}
		registered, ok := registry.Lookup(op.method, op.path)
		if !ok {
			t.Errorf("%s %s (%s) is not in the merged scope registry", op.method, op.path, op.key)
		} else if strings.Join(sortedKeys(setOf(registered)), " ") != strings.Join(sortedKeys(have), " ") {
			t.Errorf("%s %s (%s): registry accepts %v, spec declares %v", op.method, op.path, op.key, registered, op.scopes)
		}
	}
	for key := range tree.bindings {
		if !bound[key] {
			t.Errorf("x-operation-permissions binds %q, which no shipped spec declares — remove the entry or add the operation", key)
		}
	}
}

// TestShippedLegacyScopeMapValidatesAgainstTheShippedSpec runs the startup
// check on the shipped mapping: every pre-tree name maps onto permissions the
// merged spec declares, and none of them is still declared itself.
func TestShippedLegacyScopeMapValidatesAgainstTheShippedSpec(t *testing.T) {
	if err := middleware.ValidateLegacyScopeMap(loadShippedLegacyScopeMap(t), loadMergedRegistry(t)); err != nil {
		t.Fatal(err)
	}
}

// subtreePerms is every permission of n and of the nodes beneath it.
func (tree *permissionTree) subtreePerms(n *permissionNode) map[string]bool {
	out := map[string]bool{}
	for perm, owner := range tree.perms {
		if owner == n || strings.HasPrefix(owner.path, n.path+":") {
			out[perm] = true
		}
	}
	return out
}

// TestShippedLegacyScopeMapAgreesWithThePermissionTree pins each legacy entry
// to the tree's own account of it: the legacy resource must be one a node
// absorbed (its `legacy:` list), and every permission it maps to must belong to
// that node or a node beneath it — never to an ancestor, which would widen the
// old name beyond the resource it governed. What the mapping then reaches is
// pinned separately, against the pre-tree spec, by
// TestLegacyNamesReachNoMoreThanBefore.
func TestShippedLegacyScopeMapAgreesWithThePermissionTree(t *testing.T) {
	tree := loadPermissionTree(t)
	const ownershipOverrideLegacy = "ap:api_key:all:manage"

	for legacy, targets := range loadShippedLegacyScopeMap(t) {
		if legacy == ownershipOverrideLegacy {
			if len(targets) != 1 || targets[0] != constants.ScopeAPIKeyAllManage {
				t.Errorf("%s maps to %v, want [%s]", legacy, targets, constants.ScopeAPIKeyAllManage)
			}
			continue
		}
		parts := strings.Split(legacy, ":")
		resource, verb := strings.Join(parts[1:len(parts)-1], ":"), parts[len(parts)-1]
		node, ok := tree.legacy[resource]
		if !ok {
			t.Errorf("%s: resource %q is absorbed by no node in permission-tree.yaml", legacy, resource)
			continue
		}
		accepted := tree.subtreePerms(node)
		for _, target := range targets {
			if !accepted[target] {
				t.Errorf("%s (a %s on %s) maps to %q, which is not a permission of that node or of a node beneath it",
					legacy, verb, node.path, target)
			}
		}
	}
}
