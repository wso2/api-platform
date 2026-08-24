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

import React from "react";
import { createRoot } from "react-dom/client";
import AIWorkspace from "./AIWorkspace";
import type { AIWorkspaceExtension } from "./extensions";

// Loads cloud-only extensions from the `./cloud` injection seam (see
// `cloud/index.ts`) before the first render, mirroring
// `portals/api-control-plane/src/main.tsx`. Wrapped in an async function
// rather than a top-level `await` for broader build-target compatibility.
async function bootstrap() {
  const cloudExtensions: AIWorkspaceExtension[] = await import("./cloud")
    .then((module) => module.cloudExtensions)
    .catch((error) => {
      console.warn("Cloud extensions could not be loaded.", error);
      return [];
    });

  createRoot(document.getElementById("root")!).render(
    <React.StrictMode>
      <AIWorkspace extensions={cloudExtensions} />
    </React.StrictMode>,
  );
}

void bootstrap();
