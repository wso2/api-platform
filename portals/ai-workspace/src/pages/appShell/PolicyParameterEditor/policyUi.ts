/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC. and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein is strictly forbidden, unless permitted by WSO2 in accordance with
 * the WSO2 Commercial License available at http://wso2.com/licenses.
 * For specific language governing the permissions and limitations under
 * this license, please see the license as well as any agreement you've
 * entered into with WSO2 governing the purchase of this software and any
 * associated services.
 */

import { PolicyUiExtension } from './types';

/** The top-level policy definition key holding the optional form. */
export const POLICY_UI_KEY = 'x-wso2-policy-ui';

function isPlainObject(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

/**
 * Reads the optional `x-wso2-policy-ui` block. Returns undefined unless it has an
 * object `formSchema`, so a missing or malformed block keeps the standard editor.
 */
export function extractPolicyUi(raw: unknown): PolicyUiExtension | undefined {
  if (!isPlainObject(raw)) {
    return undefined;
  }
  const { formSchema, uiSchema } = raw;
  if (!isPlainObject(formSchema) || formSchema.type !== 'object') {
    return undefined;
  }
  if (uiSchema !== undefined && !isPlainObject(uiSchema)) {
    return undefined;
  }
  return { formSchema, uiSchema };
}
