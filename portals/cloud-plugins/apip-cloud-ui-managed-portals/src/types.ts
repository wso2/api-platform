/*
 * Copyright (c) 2026, WSO2 LLC (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
 * You may not alter or remove any copyright or other notice from copies of this content.
 */

// Domain types owned by this feature. Self-contained: it defines its own model
// and reaches data only through PortalPort, never a specific host's API client.

export interface ManagedPortal {
  /** Portal id — same as handle in the current backend. */
  id: string;
  /** URL-friendly slug the caller supplied on create; immutable. */
  handle: string;
  /** Display name. */
  name: string;
  /** Optional caller-supplied description. */
  description?: string;
  /**
   * Public URL of the portal, allocated by the cloud plugin. Undefined until
   * the runtime is provisioned (Phase 5 placeholder impl fills it with a
   * per-portal pending URL).
   */
  url?: string;
  /**
   * Data-plane environment whose Thunder STS backs portal-user login (portal
   * consumers subscribing to APIs). Empty on list responses (the core row's
   * list projection strips metadata by design) and populated on GET.
   * Renamed from `stsEnvironment` to name it by role (what it's for) rather
   * than by mechanism (which auth server type backs it).
   */
  loginEnvironment?: string;
  /** ISO timestamp of the last row change. */
  updatedAt?: string;
}

export interface CreateManagedPortalInput {
  handle: string;
  name: string;
  description?: string;
  /**
   * Optional. When omitted (the new Create-form default), the plugin
   * dynamically picks the org's preferred login env from
   * environments.Service.List — see the backend's Service.Create for the
   * pick order. Callers only pass this to override the server-side pick.
   */
  loginEnvironment?: string;
}

export interface UpdateManagedPortalInput {
  name?: string;
  description?: string;
  loginEnvironment?: string;
}

/**
 * A single row from the cloud plugin's GET /environments — one entry per
 * data-plane environment the org has. Used by the Create / Edit forms to
 * populate the Login environment selector so the operator picks a real value
 * rather than typing a free-form string that may not exist.
 *
 * Only the minimum shape the UI needs is modelled here; the wire payload
 * carries more fields (dnsPrefix, id, description, ...) that we ignore.
 */
export interface OrgEnvironment {
  /** Canonical env name — what the plugin's loginEnvironment field expects. */
  name: string;
  /** Human-readable name for the dropdown. Falls back to `name` when empty. */
  displayName?: string;
}

/**
 * The data contract this feature is built against. `ManagedPortalsPage`
 * constructs a real, BFF-backed port when the console's platform-api base
 * is present, falling back to an in-memory mock otherwise (tests). Neither
 * `ManagedPortalsList` nor `useManagedPortalList` ever sees anything but
 * this interface.
 */
export interface PortalPort {
  list(): Promise<ManagedPortal[]>;
  get(id: string): Promise<ManagedPortal>;
  create(input: CreateManagedPortalInput): Promise<ManagedPortal>;
  update(id: string, input: UpdateManagedPortalInput): Promise<ManagedPortal>;
  remove(id: string): Promise<void>;
  /**
   * Lists the caller-org's data-plane environments. Used by Create/Edit to
   * populate the Login environment selector. Empty list = zero environments
   * provisioned yet (org just signed up) — caller should treat that as an
   * error and show a "no environments yet" message rather than falling back
   * to a hardcoded default.
   */
  listEnvironments(): Promise<OrgEnvironment[]>;
}
