/*
 * Copyright (c) 2026, WSO2 LLC (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
 * You may not alter or remove any copyright or other notice from copies of this content.
 */

import { describe, expect, it } from 'vitest';

import { withValuePrefix } from './providerDeployApi';

describe('withValuePrefix', () => {
  it('sends the credential with the scheme the template declares', () => {
    expect(withValuePrefix('Bearer ', 'sk-test-123')).toBe('Bearer sk-test-123');
  });

  it('leaves a key that already carries the scheme alone', () => {
    expect(withValuePrefix('Bearer ', 'Bearer sk-test-123')).toBe('Bearer sk-test-123');
  });

  it('still prefixes a key whose own characters begin with the scheme text', () => {
    // "Bearerkey123" is a key, not a prefixed credential: without the separator the
    // upstream would receive no scheme at all.
    expect(withValuePrefix('Bearer ', 'Bearerkey123')).toBe('Bearer Bearerkey123');
    expect(withValuePrefix('Bearer', 'Bearer')).toBe('Bearer Bearer');
  });

  it('leaves the key untouched where the template declares no scheme', () => {
    expect(withValuePrefix(undefined, 'sk-test-123')).toBe('sk-test-123');
    expect(withValuePrefix('', 'sk-test-123')).toBe('sk-test-123');
    expect(withValuePrefix('   ', 'sk-test-123')).toBe('sk-test-123');
  });

  it('separates the scheme from the key by exactly one space', () => {
    expect(withValuePrefix('Bearer', 'sk-test-123')).toBe('Bearer sk-test-123');
    expect(withValuePrefix('Bearer   ', 'sk-test-123')).toBe('Bearer sk-test-123');
  });
});
