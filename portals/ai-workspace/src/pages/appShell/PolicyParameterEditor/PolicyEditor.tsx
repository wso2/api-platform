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

import React, { Suspense, lazy } from 'react';
import { Box, CircularProgress } from '@wso2/oxygen-ui';
import PolicyParameterEditor from './PolicyParameterEditor';
import type { PolicyParameterEditorProps } from './PolicyParameterEditor';
import RichFormBoundary from './RichFormBoundary';

// Loaded only for policies that declare a form, so the form library isn't part
// of the main bundle.
const RichPolicyForm = lazy(() => import('./rich/RichPolicyForm'));

/**
 * The params editor for a policy. A policy whose definition declares an
 * `x-wso2-policy-ui` form is rendered with that form; every other policy, or a
 * form that fails to render, uses the standard PolicyParameterEditor.
 */
const PolicyEditor: React.FC<PolicyParameterEditorProps> = (props) => {
  const { policyDefinition, existingValues } = props;
  if (!policyDefinition.ui?.formSchema) {
    return <PolicyParameterEditor {...props} />;
  }
  const policyKey = `${policyDefinition.name}@${policyDefinition.version}`;
  return (
    <RichFormBoundary key={policyKey} fallback={<PolicyParameterEditor {...props} />}>
      <Suspense
        fallback={
          <Box sx={{ display: 'flex', justifyContent: 'center', py: 4 }}>
            <CircularProgress size={28} />
          </Box>
        }
      >
        {/* A new key re-seeds the form when the saved values change. */}
        <RichPolicyForm key={`${policyKey}:${JSON.stringify(existingValues ?? null)}`} {...props} />
      </Suspense>
    </RichFormBoundary>
  );
};

export default PolicyEditor;
