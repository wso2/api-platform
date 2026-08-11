/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 * Licensed under the Apache License, Version 2.0.
 */

import React from "react";
import { createRoot } from "react-dom/client";
import AIWorkspace from "./AIWorkspace";

createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <AIWorkspace />
  </React.StrictMode>,
);
