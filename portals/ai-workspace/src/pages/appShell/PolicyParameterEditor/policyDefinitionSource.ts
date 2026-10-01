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

import { ParameterSchema, PolicyDefinition } from './types';
import { POLICY_UI_KEY, extractPolicyUi } from './policyUi';

/**
 * Custom policies already carry their full definition inline (no policy-hub
 * YAML fetch needed) — just reshape it into a PolicyDefinition.
 */
export const buildPolicyDefinitionFromCustomPolicy = (item: {
  name: string;
  version: string;
  description?: string;
  policyDefinition?: Record<string, unknown>;
}): PolicyDefinition => {
  const def = (item.policyDefinition ?? {}) as {
    description?: string;
    parameters?: ParameterSchema;
    systemParameters?: ParameterSchema;
    [POLICY_UI_KEY]?: unknown;
  };
  return {
    name: item.name,
    version: item.version,
    description: item.description || def.description || '',
    parameters: def.parameters ?? { type: 'object', properties: {} },
    systemParameters: def.systemParameters,
    ui: extractPolicyUi(def[POLICY_UI_KEY]),
  };
};
