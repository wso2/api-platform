/*
 * Copyright (c) 2026, WSO2 LLC (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
 * You may not alter or remove any copyright or other notice from copies of this content.
 */

type RuntimeConfigWindow = Window & {
  __RUNTIME_CONFIG__?: Record<string, boolean | string | undefined>;
  config?: Record<string, boolean | string | undefined>;
};

export const readRuntimeBoolean = (key: string): boolean => {
  const runtimeWindow =
    typeof window === 'undefined' ? undefined : (window as RuntimeConfigWindow);
  const value =
    runtimeWindow?.__RUNTIME_CONFIG__?.[key] ??
    runtimeWindow?.config?.[key];
  return value === true || value === 'true';
};

/** Cloud Insights sidebar entries require the BFF "cloud" named upstream. */
export const CLOUD_INSIGHTS_EXTENSION_IDS = new Set([
  'organization-insights',
  'project-insights',
]);

export const filterExtensionsForRuntime = <
  TExtension extends { id: string },
>(
  extensions: readonly TExtension[]
): TExtension[] =>
  extensions.filter(
    (extension) =>
      !CLOUD_INSIGHTS_EXTENSION_IDS.has(extension.id) ||
      readRuntimeBoolean('cloudProxyEnabled')
  );
