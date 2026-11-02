/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the
 * License at http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import type { JSX, ReactNode } from 'react';
import { Box, Typography } from '@wso2/oxygen-ui';

export type ReadOnlyBannerProps = {
  /**
   * What the strip says. A node, not a string, because whatever reported the
   * verdict owns the wording — including any emphasis and the link to wherever
   * write access is restored.
   */
  content: ReactNode;
};

/**
 * The strip that says the workspace is read-only.
 *
 * PURELY A RENDERER. It states a message it is handed and decides nothing: no
 * plan, tier, trial or subscription concept appears here, and the portal has no
 * configuration for it. Whether an organization is read-only, and what to say
 * about it, both arrive through `useResourceLimits().setReadOnly(readOnly,
 * reason)` — which in this product a cloud plugin calls and nothing else does.
 * A standalone or on-prem deployment has no supplier, so `readOnly` stays false
 * and this never renders.
 *
 * It exists because read-only already reaches every write control — they render
 * disabled, with a tooltip — but a disabled button is not an explanation. A user
 * who meets a page of dead controls needs one sentence saying why.
 */
export default function ReadOnlyBanner({
  content,
}: ReadOnlyBannerProps): JSX.Element {
  return (
    <Box
      data-testid="read-only-banner"
      sx={{
        // One line tall, full width of the content column, never squeezed by
        // the page below it.
        flexShrink: 0,
        backgroundColor: 'rgba(211, 47, 47, 0.08)',
        borderBottom: '1px solid',
        borderColor: 'error.light',
        color: 'error.dark',
        px: 2,
        py: 1,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        textAlign: 'center',
      }}
    >
      <Typography variant="body2">{content}</Typography>
    </Box>
  );
}
