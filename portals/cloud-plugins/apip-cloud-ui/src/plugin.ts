/*
 * Copyright (c) 2026, WSO2 LLC (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
 * You may not alter or remove any copyright or other notice from copies of this content.
 */

/**
 * Generic, host-agnostic plugin registration: given a set of independently
 * released feature versions (one `CloudPluginFeature` per plugin id, each
 * carrying its own version and the extensions it contributes), pick exactly
 * one release per plugin id and flatten their extensions into a single list.
 *
 * Deliberately has no opinion on what an "extension" looks like — each host
 * module (see `hosts/api-control-plane.tsx`) supplies its own concrete
 * extension type as `TExtension`, since different host portals need
 * different extension shapes (e.g. api-control-plane's extensions carry a
 * sidebar `level`/`order`, ai-workspace's don't).
 */
export type CloudPluginFeature<TExtension> = {
  id: string;
  version: string;
  releaseTag?: string;
  extensions: readonly TExtension[];
};

export function defineCloudPlugin<
  TExtension,
  const T extends CloudPluginFeature<TExtension>,
>(plugin: T): T {
  return plugin;
}

/**
 * Only one release per plugin id is activated. When multiple releases are
 * registered for the same id, `enabledVersions[id]` selects one explicitly;
 * without a selection, the highest semantic version wins.
 */
export function getCloudExtensions<TExtension>(
  plugins: readonly CloudPluginFeature<TExtension>[],
  enabledVersions: Readonly<Record<string, string>> = {}
): TExtension[] {
  const selected = new Map<string, CloudPluginFeature<TExtension>>();
  for (const plugin of plugins) {
    const requestedVersion = enabledVersions[plugin.id];
    if (requestedVersion && requestedVersion !== plugin.version) continue;
    const current = selected.get(plugin.id);
    if (!current || compareVersions(plugin.version, current.version) > 0) {
      selected.set(plugin.id, plugin);
    }
  }
  return [...selected.values()].flatMap((plugin) => [...plugin.extensions]);
}

function compareVersions(left: string, right: string): number {
  const parse = (version: string) =>
    version.split('.').map((part) => Number.parseInt(part, 10) || 0);
  const a = parse(left);
  const b = parse(right);
  for (let index = 0; index < Math.max(a.length, b.length); index += 1) {
    const difference = (a[index] ?? 0) - (b[index] ?? 0);
    if (difference !== 0) return difference;
  }
  return 0;
}
