/*
 * Copyright (c) 2026, WSO2 LLC (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
 * You may not alter or remove any copyright or other notice from copies of this content.
 */

// Feature-owned domain types; data reaches this feature only through PortalPort.

export interface ManagedPortal {
  /** Portal id; equal to `handle` in the current backend. */
  id: string;
  /** Immutable URL-friendly slug supplied on create. */
  handle: string;
  /** Display name. */
  name: string;
  /** Optional caller-supplied description. */
  description?: string;
  /** Public portal URL allocated by the cloud plugin; undefined until the runtime is provisioned. */
  url?: string;
  /** Data-plane env whose auth server backs portal-user login. Empty on list responses; populated by GET. */
  loginEnvironment?: string;
  /** ISO timestamp of the last row change. */
  updatedAt?: string;
}

export interface CreateManagedPortalInput {
  handle: string;
  name: string;
  description?: string;
  /** When omitted, the server picks the org's preferred env; pass only to override that choice. */
  loginEnvironment?: string;
}

export interface UpdateManagedPortalInput {
  name?: string;
  description?: string;
  loginEnvironment?: string;
}

/** One data-plane environment; only the fields the UI's env selector needs. */
export interface OrgEnvironment {
  /** Canonical env name; what the loginEnvironment field expects. */
  name: string;
  /** Optional label; the UI falls back to `name` when empty. */
  displayName?: string;
}

/** Data seam this feature depends on; satisfied by real (BFF) or mock (tests) implementations. */
export interface PortalPort {
  list(): Promise<ManagedPortal[]>;
  get(id: string): Promise<ManagedPortal>;
  create(input: CreateManagedPortalInput): Promise<ManagedPortal>;
  update(id: string, input: UpdateManagedPortalInput): Promise<ManagedPortal>;
  remove(id: string): Promise<void>;
  /** Empty list means the org has no environments yet; callers should surface that rather than defaulting. */
  listEnvironments(): Promise<OrgEnvironment[]>;
}
