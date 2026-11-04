/*
 * Copyright (c) 2026, WSO2 LLC (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
 * You may not alter or remove any copyright or other notice from copies of this content.
 */

// Feature-owned domain types; data reaches this feature only through PortalPort.

/**
 * Provisioning state of a managed portal, mirroring the backend plugin's
 * ManagedApiPortalStatus enum. Owned by the plugin's in-process poller,
 * NOT a live liveness signal: `active` is sticky once reached, so a portal
 * whose pod dies later still reads `active` here.
 *
 * The UI's job is to distinguish "still provisioning" from "ready to visit"
 * on Create; a rare post-provisioning outage is not something we surface.
 * `deleting` is a portal the cloud's orchestrator is tearing down: it stays
 * listed until it is gone.
 */
export type ManagedPortalStatus = 'pending' | 'active' | 'failed' | 'deleting';

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
  /** Whether portal users can create an account from the sign-in page. Empty on list responses; populated by GET. */
  selfSignup?: boolean;
  /**
   * Provisioning-lifecycle status. Missing when the backend has not been
   * upgraded to write it; treated as `active` by the UI so pre-upgrade portals
   * still show a working Visit button.
   */
  status?: ManagedPortalStatus;
  /** ISO timestamp of the last row change. */
  updatedAt?: string;
}

export interface CreateManagedPortalInput {
  handle: string;
  name: string;
  description?: string;
  /** When omitted, the server picks the org's preferred env; pass only to override that choice. */
  loginEnvironment?: string;
  /** Defaults to on when omitted. */
  selfSignup?: boolean;
}

export interface UpdateManagedPortalInput {
  name?: string;
  description?: string;
  loginEnvironment?: string;
  selfSignup?: boolean;
}

/** One data-plane environment; only the fields the UI's env selector needs. */
export interface OrgEnvironment {
  /** Canonical env name; what the loginEnvironment field expects. */
  name: string;
  /** Optional label; the UI falls back to `name` when empty. */
  displayName?: string;
  /**
   * True when openchoreo marks this env as a production tier (spec.isProduction).
   * Drives the Add-form's default selection: the picker preselects the first
   * production env so the common case (bind portal to prod) is one click. Falls
   * back to the first env in the list when no env is flagged. Missing on
   * backends that don't expose it — treated as false.
   */
  isProduction?: boolean;
}

/** Data seam this feature depends on; satisfied by real (BFF) or mock (tests) implementations. */
export interface PortalPort {
  list(): Promise<ManagedPortal[]>;
  get(id: string): Promise<ManagedPortal>;
  create(input: CreateManagedPortalInput): Promise<ManagedPortal>;
  update(id: string, input: UpdateManagedPortalInput): Promise<ManagedPortal>;
  /** `pending` is true when the portal is torn down in the background and reads as deleting until it is gone. */
  remove(id: string): Promise<{ pending: boolean }>;
  /** Empty list means the org has no environments yet; callers should surface that rather than defaulting. */
  listEnvironments(): Promise<OrgEnvironment[]>;
}
