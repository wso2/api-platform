/*
 * Copyright (c) 2026, WSO2 LLC (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
 * You may not alter or remove any copyright or other notice from copies of this content.
 */

/*
 * The API Platform console features this page reports, in the console's own
 * terms (`portals/api-control-plane/src/featureUsage/api-platform-taxonomy.yaml`).
 * Reported through the host Port's `trackFeature`, which drops codes the
 * taxonomy does not declare.
 */

export const PORTAL_ADD_OPEN = 'apip.developer-portal.portal-add-open';
export const PORTAL_CREATE_CLICK = 'apip.developer-portal.portal-create-click';

/** Where the add-portal form was opened from: the taxonomy's `entry_point` for `PORTAL_ADD_OPEN`. */
export type PortalAddEntryPoint = 'portals-list' | 'portals-empty-state';
