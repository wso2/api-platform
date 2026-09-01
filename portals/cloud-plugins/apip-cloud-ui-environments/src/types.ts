/*
 * Copyright (c) 2026, WSO2 LLC (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
 * You may not alter or remove any copyright or other notice from copies of this content.
 */

// Domain types owned by this feature. Self-contained: it defines its own model and
// reaches data only through EnvironmentPort, never a specific host's API client.

export interface CloudEnvironment {
  id: string;
  name: string;
  displayName: string;
  isProduction: boolean;
  createdAt: string;
}

export interface CreateEnvironmentInput {
  displayName: string;
  isProduction: boolean;
}

/**
 * The data contract this feature is built against. Today `EnvironmentsPage`
 * constructs an in-memory `createMockEnvironmentPort()` internally (no real
 * backend exists yet) — this interface is what a real implementation swaps in
 * for, without any change to `EnvironmentsList`/`useEnvironmentList`.
 */
export interface EnvironmentPort {
  list(): Promise<CloudEnvironment[]>;
  create(input: CreateEnvironmentInput): Promise<CloudEnvironment>;
  remove(envId: string): Promise<void>;
}
