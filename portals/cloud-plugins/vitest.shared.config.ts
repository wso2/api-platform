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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

// One vitest config for every cloud plugin, run from the plugin's own folder
// (`vitest run --config ../vitest.shared.config.ts`). The plugins resolve their
// dependencies from the API Control Plane's node_modules, linked here as
// ./node_modules. A plugin's own vitest.setup.ts, if it has one, is loaded.
import { existsSync } from 'node:fs';
import { resolve } from 'node:path';
import react from '@vitejs/plugin-react';
import { defineConfig } from 'vitest/config';

const setup = resolve(process.cwd(), 'vitest.setup.ts');

export default defineConfig({
  plugins: [react()],
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: existsSync(setup) ? [setup] : [],
    // Oxygen UI imports prismjs paths Node's ESM loader cannot resolve; let
    // Vite transform it (and its MUI deps), as the console's own config does.
    server: { deps: { inline: [/@wso2\/oxygen-ui/, /@mui\//] } },
  },
});
