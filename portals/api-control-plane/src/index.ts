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

// Public entry point for consuming this app as a workspace-linked package (see
// docs/remote-app-wrapper-app-architecture.md). Because package.json's "main"
// points here (at TS source, not a build output), a host importing
// "api-control-plane" resolves straight through to these actual files —
// hot-reloading edits across both apps in a single dev server, with no
// separate "build api-control-plane, then run the host" step.
export { default as App } from './App';
export { loadRuntimeConfigScripts } from './config/loadRuntimeConfigScripts';
