/*
 * Copyright (c) 2026, WSO2 LLC (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
 * You may not alter or remove any copyright or other notice from copies of this content.
 */

import { describe, expect, it } from 'vitest';

import { billingOrganizationUrl } from './trialApi';

describe('billingOrganizationUrl', () => {
  it('resolves against a portal mounted under a sub-path', () => {
    expect(billingOrganizationUrl('/ai-workspace/')).toBe(
      '/ai-workspace/proxy/billing/organization?product=api-platform'
    );
  });

  it('resolves against a portal mounted at the root', () => {
    expect(billingOrganizationUrl('/')).toBe(
      '/proxy/billing/organization?product=api-platform'
    );
  });

  it('tolerates a base without a trailing slash', () => {
    expect(billingOrganizationUrl('/ai-workspace')).toBe(
      '/ai-workspace/proxy/billing/organization?product=api-platform'
    );
  });

  it('falls back to the root when no base is available', () => {
    expect(billingOrganizationUrl(undefined)).toBe(
      '/proxy/billing/organization?product=api-platform'
    );
  });
});
