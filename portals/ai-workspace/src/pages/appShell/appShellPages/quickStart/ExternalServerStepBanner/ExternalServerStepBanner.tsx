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

import { useState } from 'react';
import {
  Box,
  Card,
  CircularProgress,
  IconButton,
  Stack,
  Typography,
} from '@wso2/oxygen-ui';
import { ArrowRight, Rocket, SlidersHorizontal, X } from '@wso2/oxygen-ui-icons-react';
import QuickStartCompletedIcon from '../../../../../assets/images/quickStart/QuickStartCompletedIcon';

export type ExternalServerStepBannerStepId =
  | 'add-policies'
  | 'deploy-to-gateway';

export type ExternalServerStepBannerProps = {
  serverName?: string;
  hasPolicies: boolean;
  hasDeployments: boolean;
  onStepClick?: (stepId: ExternalServerStepBannerStepId) => void;
};

const TOTAL_STEPS = 3;

const stepButtonSx = {
  border: '1px solid',
  borderColor: '#d4d8e3',
  backgroundColor: '#ffffff',
  color: '#1f2430',
  display: 'inline-flex',
  alignItems: 'center',
  gap: 1,
  px: 1.5,
  py: 1.1,
  borderRadius: '10px',
  minWidth: 'fit-content',
  cursor: 'pointer',
  flexShrink: 0,
  transition: 'border-color 0.2s ease, box-shadow 0.2s ease, transform 0.2s ease',
  '&:hover': {
    borderColor: '#ff6701',
    boxShadow: '0 4px 12px rgba(19, 93, 138, 0.12)',
    transform: 'translateY(-1px)',
  },
};

export default function ExternalServerStepBanner({
  serverName,
  hasPolicies,
  hasDeployments,
  onStepClick,
}: ExternalServerStepBannerProps) {
  const [isVisible, setIsVisible] = useState(true);

  const resolvedServerName = serverName?.trim() || 'External Server';
  const completedSteps = 1 + (hasPolicies ? 1 : 0) + (hasDeployments ? 1 : 0);
  const progressValue = (completedSteps / TOTAL_STEPS) * 100;

  if (!isVisible) return null;

  return (
    <Card
      sx={{
        width: '100%',
        position: 'relative',
        overflow: 'hidden',
        border: '1px solid',
        borderColor: '#ff6701',
        borderRadius: '16px',
        px: { xs: 1, md: 1.5 },
        py: { xs: 1, md: 1.5 },
      }}
    >
      <IconButton
        size="small"
        onClick={() => setIsVisible(false)}
        aria-label="Close external server setup steps banner"
        sx={{ position: 'absolute', top: 8, right: 8, zIndex: 1, color: '#5f6472' }}
      >
        <X size={16} />
      </IconButton>

      <Stack
        direction={{ xs: 'column', md: 'row' }}
        spacing={{ xs: 2, md: 2.5 }}
        alignItems={{ xs: 'flex-start', md: 'center' }}
      >
        {/* Circular progress */}
        <Box sx={{ position: 'relative', width: 64, height: 64, flexShrink: 0 }}>
          <CircularProgress
            variant="determinate"
            value={100}
            size={64}
            thickness={4}
            sx={{ color: '#ffd7bd', position: 'absolute', inset: 0 }}
          />
          <CircularProgress
            variant="determinate"
            value={progressValue}
            size={64}
            thickness={4}
            sx={{ color: '#ff6701', position: 'absolute', inset: 0 }}
          />
          <Box
            sx={{
              position: 'absolute',
              inset: 0,
              display: 'flex',
              flexDirection: 'column',
              alignItems: 'center',
              justifyContent: 'center',
              gap: 0.5,
            }}
          >
            <Typography sx={{ fontWeight: 700, lineHeight: 1, fontSize: 18 }}>
              {`${completedSteps}/${TOTAL_STEPS}`}
            </Typography>
            <Typography sx={{ lineHeight: 1.1, fontSize: 10, color: '#667085' }}>
              Steps
            </Typography>
          </Box>
        </Box>

        <Stack spacing={0.6} sx={{ minWidth: 0, flex: 1 }}>
          <Box sx={{ gap: 0.1, display: 'flex', flexDirection: 'column' }}>
            <Typography sx={{ fontSize: 16, fontWeight: 500, lineHeight: 1.35 }}>
              <Box component="span">Successfully Created </Box>
              <Box component="span" sx={{ fontWeight: 700 }}>{resolvedServerName}</Box>
            </Typography>
            <Typography sx={{ mt: 0.35, fontSize: 12, color: '#667085', lineHeight: 1.4 }}>
              Click each step to configure and deploy your MCP Proxy.
            </Typography>
          </Box>

          <Box
            sx={{
              display: 'flex',
              alignItems: 'center',
              gap: 1,
              overflowX: 'auto',
              overflowY: 'hidden',
              p: 0.5,
              minWidth: 0,
              scrollbarWidth: 'thin',
            }}
          >
            {/* Step 2: Configure Policies */}
            <Box
              component="button"
              type="button"
              onClick={() => onStepClick?.('add-policies')}
              sx={stepButtonSx}
            >
              {hasPolicies ? <QuickStartCompletedIcon size={16} /> : <SlidersHorizontal size={16} />}
              <Typography sx={{ fontSize: 14, fontWeight: 500, whiteSpace: 'nowrap' }}>
                Configure Policies
              </Typography>
              <ArrowRight size={16} color="#9b9b9b" />
            </Box>

            {/* Step 3: Deploy to Gateway & Test */}
            <Box
              component="button"
              type="button"
              onClick={() => onStepClick?.('deploy-to-gateway')}
              sx={stepButtonSx}
            >
              {hasDeployments ? <QuickStartCompletedIcon size={16} /> : <Rocket size={16} />}
              <Typography sx={{ fontSize: 14, fontWeight: 500, whiteSpace: 'nowrap' }}>
                Deploy to Gateway &amp; Test
              </Typography>
              <ArrowRight size={16} color="#9b9b9b" />
            </Box>
          </Box>
        </Stack>
      </Stack>
    </Card>
  );
}
