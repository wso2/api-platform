/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import React from 'react';
import { Box, IconButton, Tooltip, Typography } from '@wso2/oxygen-ui';
import { HelpCircle } from '@wso2/oxygen-ui-icons-react';
import policiesDragGif from '../../assets/images/policies-drag.gif';

export default function PoliciesReorderHelp() {
  return (
    <Tooltip
      placement="bottom-start"
      title={
        <Box>
          <Box
            component="img"
            src={policiesDragGif}
            alt="Reordering policies by dragging and dropping"
            sx={{ display: 'block', width: '100%', height: 'auto' }}
          />
          <Box sx={{ px: 2.25, pt: 1.75, pb: 2 }}>
            <Typography
              sx={{
                color: '#FF7300',
                fontSize: 18,
                fontWeight: 600,
                lineHeight: 1.35,
                mb: 0.5,
              }}
            >
              Add and reorder your policies
            </Typography>
            <Typography
              sx={{
                color: 'rgba(255, 255, 255, 0.82)',
                fontSize: 14,
                fontWeight: 400,
                lineHeight: 1.45,
              }}
            >
              Add guardrails and policies, then drag and drop them to change
              their execution order globally or for an individual resource.
            </Typography>
          </Box>
        </Box>
      }
      slotProps={{
        tooltip: {
          sx: {
            p: 0,
            width: 360,
            maxWidth: 'none',
            overflow: 'hidden',
            bgcolor: '#2C2726',
            border: '1px solid rgba(255, 135, 61, 0.38)',
            borderRadius: '12px',
            boxShadow: '0 8px 24px rgba(0, 0, 0, 0.38)',
          },
        },
        popper: {
          modifiers: [
            {
              name: 'offset',
              options: { offset: [-10, 8] },
            },
          ],
        },
      }}
    >
      <IconButton
        size="small"
        aria-label="Learn how to reorder policies"
        sx={{ p: 0.25, color: 'text.secondary' }}
      >
        <HelpCircle size={16} />
      </IconButton>
    </Tooltip>
  );
}
