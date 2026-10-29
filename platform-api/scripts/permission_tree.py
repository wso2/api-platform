#!/usr/bin/env python3
# Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
#
# WSO2 LLC. licenses this file to you under the Apache License,
# Version 2.0 (the "License"); you may not use this file except
# in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing,
# software distributed under the License is distributed on an
# "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
# KIND, either express or implied.  See the License for the
# specific language governing permissions and limitations
# under the License.
"""Generate the OpenAPI security blocks from resources/permission-tree.yaml.

    python3 scripts/permission_tree.py            # rewrite specs in place
    python3 scripts/permission_tree.py --check    # exit 1 if anything is out of date

permission-tree.yaml declares the permissions (node path + action, joined with
":") and binds every operation to the permissions it accepts directly under
x-operation-permissions, keyed by operationId — or by "METHOD /path" for an
operation that has none (the event-gateway plugin spec). This script turns each
binding into the operation's accepted-scope list:

  1. The binding names the operation's node and action: the deepest tree
     permission in the list is the node, and its non-manage action (if any) is
     the action — CreateRESTAPI binds to ap:api:rest + create.
  2. Inheritance: the list gains that action's permission and :manage on the
     node itself, then :manage on every ancestor that defines it — and, for a
     read, :read on every ancestor too. Only read and manage flow down the tree;
     every other verb (create, update, delete, restore, undeploy, ...) applies
     to its own node only. So ap:api:rest:build:create, ap:api:rest:build:manage,
     ap:api:rest:manage and ap:api:manage admit CreateBuild, but
     ap:api:rest:create does not: creating a REST API is not deploying it.
     ScopeEnforcer matches the list by exact string equality and never walks the
     tree itself, which is why every admitting permission must be written out.
  3. A permission of a node outside that chain that the binding lists is a
     deliberate cross-resource grant (GET /rest-apis/{id}/gateways also accepts
     ap:gateway:read) and is kept; the API-key ownership modifier
     (ap:credential:api_key:all:manage) is kept wherever it is listed.

The script edits the YAML as text so comments and formatting outside the
security lists are untouched. The main spec's OAuth2 scope catalog is rewritten
to list exactly the permissions its operations accept, keeping an existing
description and generating one for a new permission.

Pre-tree scope names are never written to the specs. They live in
resources/legacy-scope-mapping.yaml, which names the tree permission each one
grants; the Platform API translates them per request. This script validates
that file: every legacy name must resolve through a node's `legacy` list, every
permission it maps to must be one the inheritance rule accepts for that node
and verb, and no legacy name may still be declared by a spec.

It also keeps the tree's own x-authorization-policy.aggregate_members current
and writes the tree as a TypeScript module for the AI Workspace SPA
(portals/ai-workspace/src/auth/permissionTree.generated.ts), whose client-side
scope gates derive the same accepted lists from it.
"""
import argparse
import os
import re
import sys

import yaml

# The libyaml-backed loader is several times faster on the ~570 KB OpenAPI spec;
# fall back to the pure-Python one where PyYAML was built without libyaml.
_YAML_LOADER = getattr(yaml, "CSafeLoader", yaml.SafeLoader)


def load_yaml(stream):
    return yaml.load(stream, Loader=_YAML_LOADER)

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.dirname(HERE)
TREE = os.path.join(ROOT, "resources", "permission-tree.yaml")
LEGACY_MAP = os.path.join(ROOT, "resources", "legacy-scope-mapping.yaml")
SPECS = [
    # (path, security scheme name, has a scope catalog)
    (os.path.join(ROOT, "resources", "openapi.yaml"), "OAuth2Security", True),
    (os.path.join(ROOT, "plugins", "eventgateway", "openapi.yaml"), "OAuth2SecurityEventGateway", False),
]
# The AI Workspace SPA evaluates its scope gates client-side; it imports the
# tree from this generated module so its derivation matches the spec's.
TREE_TS = os.path.join(os.path.dirname(ROOT), "portals", "ai-workspace", "src", "auth",
                       "permissionTree.generated.ts")
METHODS = ("get", "put", "post", "delete", "patch", "head", "options", "trace")
# The only actions an ancestor's permission passes down to the nodes below it.
INHERITED_ACTIONS = ("read", "manage")
OWNERSHIP_MODIFIER_LEGACY = "ap:api_key:all:manage"
OWNERSHIP_MODIFIER = "ap:credential:api_key:all:manage"
# Catalog description for a permission no operator has described yet.
ACTION_PHRASES = {
    "read": "Read {}", "manage": "Full access to {}", "create": "Create {}",
    "update": "Update {}", "delete": "Delete {}", "restore": "Restore {}",
    "undeploy": "Undeploy {}", "regenerate": "Regenerate {}",
}


class Node:
    def __init__(self, path, raw, parent):
        self.path = path                      # tuple of handles, e.g. ("ap", "api", "rest")
        self.name = raw.get("name", path[-1])
        self.actions = list(raw.get("actions", []))
        self.legacy = list(raw.get("legacy", []))
        self.plugin = raw.get("plugin") or (parent.plugin if parent else None)
        self.parent = parent
        self.children = []

    def perm(self, action):
        return ":".join(self.path + (action,))

    def chain(self):
        n = self
        while n is not None:
            yield n
            n = n.parent

    def full_name(self):
        return " ".join(n.name for n in reversed(list(self.chain())) if n.parent is not None) or self.name


class Tree:
    def __init__(self, path=TREE):
        self.doc = load_yaml(open(path))
        delim = self.doc["resource_server"]["delimiter"]
        assert delim == ":", "generator assumes ':' delimiter"
        self.nodes, self.legacy = [], {}

        def walk(items, parent):
            for raw in items:
                n = Node((parent.path if parent else ()) + (raw["handle"],), raw, parent)
                if parent:
                    parent.children.append(n)
                self.nodes.append(n)
                for l in n.legacy:
                    if l in self.legacy:
                        sys.exit(f"legacy resource {l!r} mapped to two nodes")
                    self.legacy[l] = n
                walk(raw.get("resources", []), n)

        walk(self.doc["tree"], None)
        self.perms = {n.perm(a): (n, a) for n in self.nodes for a in n.actions}
        if OWNERSHIP_MODIFIER not in self.perms:
            sys.exit(f"permission-tree.yaml does not define the ownership modifier {OWNERSHIP_MODIFIER}")
        self.bindings = self.doc.get("x-operation-permissions") or {}
        if not self.bindings:
            sys.exit("permission-tree.yaml: x-operation-permissions is empty")
        policy = self.doc.get("x-authorization-policy") or {}
        self.aggregates = list((policy.get("aggregate_members") or {}).keys())

    def inherit(self, node, action):
        """The permissions the inheritance rule grants for `action` on `node`:
        that action and :manage on the node itself, then :manage — and, for a
        read, :read — on every ancestor defining it. No other verb flows down:
        ap:api:rest:create never admits an operation on a build, deployment or
        key. Most specific first."""
        out = []
        for x in node.chain():
            if action in x.actions and (x is node or action in INHERITED_ACTIONS):
                out.append(x.perm(action))
            if action != "manage" and "manage" in x.actions:
                out.append(x.perm("manage"))
        return out

    def bind(self, key, explicit):
        """The node and action an operation is bound to, from the permissions it
        lists directly: the deepest tree permission names the node, and its
        non-manage action (if it lists one) the action."""
        tree, unknown = [], []
        for s in explicit:
            if s == OWNERSHIP_MODIFIER:
                continue
            if s in self.perms:
                tree.append(self.perms[s])
            else:
                unknown.append(s)
        if unknown:
            sys.exit(f"{key}: {unknown} are not permissions of permission-tree.yaml")
        if not tree:
            sys.exit(f"{key}: binds to no tree permission")
        depth = max(len(n.path) for n, _ in tree)
        deepest = [(n, a) for n, a in tree if len(n.path) == depth]
        nodes = {n.path for n, _ in deepest}
        if len(nodes) != 1:
            sys.exit(f"{key}: binds to two nodes at the same depth: {sorted(':'.join(p) for p in nodes)}")
        node = deepest[0][0]
        actions = sorted({a for _, a in deepest if a != "manage"})
        if len(actions) > 1:
            sys.exit(f"{key}: binds {':'.join(node.path)} to two actions {actions}")
        return node, (actions[0] if actions else "manage")

    def accepted(self, key, explicit):
        """The operation's full accepted list: the inheritance rule applied to
        its binding, then every cross-resource grant and the ownership modifier
        it lists, in the order listed."""
        node, action = self.bind(key, explicit)
        out = self.inherit(node, action)
        chain = {x.path for x in node.chain()}
        for s in explicit:
            if s in out:
                continue
            if s == OWNERSHIP_MODIFIER:
                out.append(s)
                continue
            n, a = self.perms[s]
            if n.path in chain:
                sys.exit(f"{key}: lists {s}, which is on the path of its binding "
                         f"{':'.join(node.path)}:{action} but not what the inheritance rule grants there")
            out.append(s)
        return out, (node, action)

    def aggregate_members(self, perm):
        """Every permission below `perm`'s node that `perm` admits."""
        node, action = self.perms[perm]
        return sorted(p for p, (n, a) in self.perms.items()
                      if n is not node and node.path == n.path[:len(node.path)] and perm in self.inherit(n, a))


def legacy_action(tree, legacy):
    """(node, verb) for a pre-tree scope, or None for the ownership modifier."""
    if legacy in (OWNERSHIP_MODIFIER, OWNERSHIP_MODIFIER_LEGACY):
        return None
    parts = legacy.split(":")
    res, verb = ":".join(parts[1:-1]), parts[-1]
    if res not in tree.legacy:
        sys.exit(f"legacy scope {legacy!r}: resource {res!r} is absorbed by no node in permission-tree.yaml")
    return tree.legacy[res], verb


PATH_RE = re.compile(r"^  (/\S*):\s*$")
METHOD_RE = re.compile(r"^    (" + "|".join(METHODS) + r"):\s*$")


def operation_keys(text):
    """(METHOD, path) -> binding key for every operation of a spec."""
    spec = load_yaml(text)
    keys = {}
    for path, item in spec["paths"].items():
        for meth, op in item.items():
            if meth in METHODS:
                keys[(meth.upper(), path)] = op.get("operationId") or f"{meth.upper()} {path}"
    return keys


def rewrite_spec(text, scheme, tree, bound, missing):
    keys = operation_keys(text)
    lines = text.split("\n")
    out, i = [], 0
    path = method = None
    in_paths = False
    req_re = re.compile(r"^(\s*)- " + re.escape(scheme) + r":\s*$")
    while i < len(lines):
        line = lines[i]
        if line.startswith("paths:"):
            in_paths = True
        elif re.match(r"^\S", line) and not line.startswith("paths:"):
            in_paths = False
        if in_paths:
            m = PATH_RE.match(line)
            if m:
                path, method = m.group(1), None
            m = METHOD_RE.match(line)
            if m:
                method = m.group(1)
        m = req_re.match(line)
        if in_paths and m and method:
            out.append(line)
            i += 1
            indent = m.group(1) + "    "
            while i < len(lines):
                mm = re.match(r"^(\s*)- (ap:\S+)\s*$", lines[i])
                if not mm or len(mm.group(1)) <= len(m.group(1)):
                    break
                indent = mm.group(1)
                i += 1
            key = keys[(method.upper(), path)]
            explicit = tree.bindings.get(key)
            if explicit is None:
                missing.append(key)
                continue
            new, b = tree.accepted(key, explicit)
            bound[key] = b
            out.extend(f"{indent}- {s}" for s in new)
            continue
        out.append(line)
        i += 1
    return "\n".join(out)


def validate_legacy_map(path, tree, declared):
    """Check legacy-scope-mapping.yaml against the tree and the rewritten specs."""
    doc = load_yaml(open(path))
    scopes = (doc or {}).get("scopes") or {}
    if not scopes:
        sys.exit(f"{os.path.relpath(path, ROOT)}: no scopes")
    for old, targets in sorted(scopes.items()):
        if old in tree.perms:
            sys.exit(f"{old!r} is a tree permission, not a legacy name")
        if old in declared:
            sys.exit(f"legacy scope {old!r} is still declared by an operation — remove it from the spec or the mapping")
        cls = legacy_action(tree, old)
        # A legacy name maps only onto permissions of the node that absorbed its
        # resource or of the nodes beneath it — never onto an ancestor's, which
        # would widen it. That lets an entry stay narrower than the node's
        # :manage where the old name never governed the sub-resources
        # (ap:api_portal:manage onto the portal's create/read/update/delete;
        # ap:websub_api:manage onto the API, its publications and HMAC secrets).
        # internal/server/testdata/pre-tree-scopes.yaml and
        # TestLegacyNamesReachNoMoreThanBefore pin the resulting reach.
        accepted = ([OWNERSHIP_MODIFIER] if cls is None else
                    [p for p, (n, _) in tree.perms.items() if n.path[:len(cls[0].path)] == cls[0].path])
        if not isinstance(targets, list) or not targets:
            sys.exit(f"legacy scope {old!r} maps to no permission")
        for t in targets:
            if t not in accepted:
                sys.exit(f"legacy scope {old!r} maps to {t!r}, which is not a permission of the node that absorbed it or of a node beneath it")
            if t not in declared:
                sys.exit(f"legacy scope {old!r} maps to {t!r}, which no operation declares")
    return scopes


def rewrite_catalog(text, tree, used):
    """List exactly the permissions the spec's operations accept, keeping the
    description of one already listed."""
    lines = text.split("\n")
    start = next(i for i, l in enumerate(lines) if l.strip() == "scopes:" and "clientCredentials" in "".join(lines[i - 3:i]))
    end = start + 1
    while end < len(lines) and re.match(r"^\s{12}\S", lines[end]):
        end += 1
    existing = {}
    for l in lines[start + 1:end]:
        k, _, v = l.strip().partition(": ")
        existing[k] = v
    if OWNERSHIP_MODIFIER in used and OWNERSHIP_MODIFIER not in existing:
        existing[OWNERSHIP_MODIFIER] = "Manage API keys created by any user in the organization"
    for p in used:
        if p not in existing:
            n, a = tree.perms[p]
            existing[p] = ACTION_PHRASES.get(a, a.capitalize() + " {}").format(n.full_name())
    for k in list(existing):
        if k not in used:
            del existing[k]          # catalog lists exactly what operations accept
    block = [f"            {k}: {existing[k]}" for k in sorted(existing)]
    return "\n".join(lines[:start + 1] + block + lines[end:])


def rewrite_aggregates(text, tree):
    """Keep x-authorization-policy.aggregate_members equal to what the tree grants."""
    if not tree.aggregates:
        return text
    m = re.search(r"^  aggregate_members:\n((?:    .*\n)*)", text, re.M)
    if not m:
        sys.exit("permission-tree.yaml: aggregate_members block not found")
    block = []
    for perm in tree.aggregates:
        if perm not in tree.perms:
            sys.exit(f"aggregate_members: {perm!r} is not a tree permission")
        block.append(f"    {perm}:")
        block.extend(f"    - {p}" for p in tree.aggregate_members(perm))
    return text[:m.start(1)] + "\n".join(block) + "\n" + text[m.end(1):]


TS_LICENSE = """/*
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
"""


def render_tree_ts(tree, legacy_scopes):
    """The tree as a TypeScript module for the AI Workspace's client-side scope gates."""
    out = [TS_LICENSE, "/**",
           " * GENERATED FILE — do not edit.",
           " *",
           " * Produced by platform-api/scripts/permission_tree.py from",
           " * platform-api/resources/permission-tree.yaml and",
           " * platform-api/resources/legacy-scope-mapping.yaml. Edit those, not this file.",
           " *",
           " * The permission tree as data for the AI Workspace's client-side scope gates",
           " * (src/auth/permissions.ts): the nodes and the actions each defines, the",
           " * pre-tree resource each node absorbed, the pre-tree scope names and what",
           " * each grants, and the API-key ownership override's two names. permissions.ts",
           " * derives from these exactly the accepted lists the Platform API enforces.",
           " */", "",
           "/** Every node of the permission tree, keyed by its permission prefix, with the actions it defines. */",
           "export const PERMISSION_TREE_NODES: Readonly<Record<string, readonly string[]>> = {"]
    for n in tree.nodes:
        out.append(f"  '{':'.join(n.path)}': [{', '.join(repr(a) for a in n.actions)}],")
    out += ["};", "",
            "/** Pre-tree scope resources (the part between `ap:` and the verb) and the node each one became. */",
            "export const LEGACY_SCOPE_RESOURCES: Readonly<Record<string, string>> = {"]
    for l, n in sorted(tree.legacy.items()):
        out.append(f"  '{l}': '{':'.join(n.path)}',")
    out += ["};", "",
            "/**",
            " * Pre-tree scope names the Platform API still accepts and the permissions each",
            " * one grants (resources/legacy-scope-mapping.yaml). The API expands a request's",
            " * held scopes through this before matching; checkPermission expands a user's",
            " * scopes the same way, so an old token or role is offered what the API admits.",
            " */",
            "export const LEGACY_SCOPES: Readonly<Record<string, readonly string[]>> = {"]
    for name in sorted(legacy_scopes):
        out.append(f"  '{name}': [{', '.join(repr(s) for s in legacy_scopes[name])}],")
    out += ["};", "",
            "/** The API-key ownership modifier, by its permission-tree name and its pre-tree name. */",
            f"export const OWNERSHIP_MODIFIER_SCOPES = ['{OWNERSHIP_MODIFIER}', '{OWNERSHIP_MODIFIER_LEGACY}'] as const;"]
    return "\n".join(out) + "\n"


def declared_scopes(text):
    spec = load_yaml(text)
    used = set()
    for item in spec["paths"].values():
        for meth, op in item.items():
            if meth in METHODS:
                for req in op.get("security") or []:
                    for v in req.values():
                        used.update(v)
    return used


def write_if_changed(path, new, check):
    current = open(path).read() if os.path.exists(path) else None
    if new == current:
        return False
    if not check:
        open(path, "w").write(new)
    return True


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--check", action="store_true")
    args = ap.parse_args()
    tree = Tree()
    stale = False
    declared, bound, missing = set(), {}, []
    for path, scheme, catalog in SPECS:
        text = open(path).read()
        new = rewrite_spec(text, scheme, tree, bound, missing)
        if missing:
            sys.exit(f"{os.path.relpath(path, ROOT)}: no x-operation-permissions entry for {missing}")
        used = declared_scopes(new)
        declared |= used
        if catalog:
            new = rewrite_catalog(new, tree, used)
        stale |= write_if_changed(path, new, args.check)
        # Parse the spec once for the count — calling operation_keys() inside the
        # generator re-parsed the whole spec for every bound operation.
        spec_keys = set(operation_keys(text).values())
        print(f"{os.path.relpath(path, ROOT)}: {sum(1 for k in bound if k in spec_keys)} operations bound", file=sys.stderr)
    dead = [key for key in tree.bindings if key not in bound]
    if dead:
        sys.exit(f"x-operation-permissions binds operations no spec declares: {dead} — remove them from permission-tree.yaml")
    unused = sorted(p for p in tree.perms if p not in declared and p != OWNERSHIP_MODIFIER)
    if unused:
        sys.exit(f"permission-tree.yaml defines permissions no operation accepts: {unused} — remove the node or action")
    legacy_scopes = validate_legacy_map(LEGACY_MAP, tree, declared)
    print(f"{os.path.relpath(LEGACY_MAP, ROOT)}: {len(legacy_scopes)} legacy scopes map onto declared permissions", file=sys.stderr)
    stale |= write_if_changed(TREE, rewrite_aggregates(open(TREE).read(), tree), args.check)
    stale |= write_if_changed(TREE_TS, render_tree_ts(tree, legacy_scopes), args.check)
    print(f"{os.path.relpath(TREE_TS, ROOT)}: {len(tree.nodes)} nodes, {len(tree.legacy)} legacy resources, {len(legacy_scopes)} legacy scopes", file=sys.stderr)
    if args.check and stale:
        sys.exit("specs are out of date — run scripts/permission_tree.py")


if __name__ == "__main__":
    main()
