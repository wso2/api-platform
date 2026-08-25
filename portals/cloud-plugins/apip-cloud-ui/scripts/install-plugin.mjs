/*
 * Copyright (c) 2026, WSO2 LLC (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
 * You may not alter or remove any copyright or other notice from copies of this content.
 */

// Fills in per-feature version placeholders in a copied host file.
//
// api-control-plane's extension seam is native (see api-platform's
// portals/api-control-plane/src/extensions.tsx) — main.tsx already imports
// `cloudExtensions` from a checked-in `src/cloud` stub unconditionally, so
// the Dockerfile only needs to overlay this package's host file over that
// stub. No App.tsx/main.tsx/sidebar source patching happens here, unlike the
// older ai-workspace `install-plugin.mjs`, which pre-dates ai-workspace
// having that same native seam.
//
// Usage: node install-plugin.mjs <path-to-host.tsx> [FEATURE_VERSION=x.y.z ...]
// Each KEY=value replaces the literal token __KEY__ in the target file. A
// build with zero registered features (the case today) passes zero version
// arguments and this is a no-op copy.
import { readFile, writeFile } from 'node:fs/promises';

const [hostFile, ...versionArgs] = process.argv.slice(2);
if (!hostFile) {
  throw new Error(
    'Usage: install-plugin.mjs <path-to-host.tsx> [FEATURE_VERSION=x.y.z ...]'
  );
}

let source = await readFile(hostFile, 'utf8');

for (const arg of versionArgs) {
  const separatorIndex = arg.indexOf('=');
  if (separatorIndex <= 0) {
    throw new Error(`Malformed version argument "${arg}" (expected KEY=value)`);
  }
  const key = arg.slice(0, separatorIndex);
  const value = arg.slice(separatorIndex + 1);
  const token = `__${key}__`;
  if (!source.includes(token)) {
    throw new Error(`Placeholder ${token} not found in ${hostFile}`);
  }
  source = source.split(token).join(value);
}

await writeFile(hostFile, source, 'utf8');
