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

import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  Box,
  Button,
  Card,
  CircularProgress,
  IconButton,
  Stack,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import { ArrowRight, X } from '@wso2/oxygen-ui-icons-react';
import { getLLMProviders } from '../../../../../apis/llmProviderApis';
import { useAppShell } from '../../../../../contexts/AppShellContext';
import { PLATFORM_API_BASE_URL } from '../../../../../config.env';
import { buildOrgPath } from '../../../../../utils/projectRouting';

export type AIGatewayStepBannerProps = {
  gatewayDisplayName?: string;
  isActive: boolean;
  onFinish?: () => void;
  onDismiss?: () => void;
};

const TOTAL_STEPS = 2;

export default function AIGatewayStepBanner({
  gatewayDisplayName,
  isActive,
  onFinish,
  onDismiss,
}: AIGatewayStepBannerProps) {
  const [isVisible, setIsVisible] = useState(true);
  const [providerCount, setProviderCount] = useState(0);

  const { currentOrganization } = useAppShell();
  const navigate = useNavigate();
  const organizationId = currentOrganization?.uuid ?? '';

  const resolvedDisplayName = gatewayDisplayName?.trim() || 'AI Gateway';
  const completedSteps = isActive ? 2 : 1;
  const progressValue = (completedSteps / TOTAL_STEPS) * 100;
  const isProviderQuotaReached = false;
  const newProviderPath = buildOrgPath(currentOrganization, '/service-provider/new');

  useEffect(() => {
    if (!isActive || !organizationId) return;

    let isCancelled = false;
    getLLMProviders(organizationId, PLATFORM_API_BASE_URL)
      .then((result) => {
        if (!isCancelled) setProviderCount(result?.list?.length ?? 0);
      })
      .catch(() => {
        if (!isCancelled) setProviderCount(0);
      });

    return () => { isCancelled = true; };
  }, [isActive, organizationId, PLATFORM_API_BASE_URL]);

  const handleDismiss = () => {
    setIsVisible(false);
    onDismiss?.();
  };

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
        onClick={handleDismiss}
        aria-label="Close gateway setup banner"
        sx={{
          position: 'absolute',
          top: 8,
          right: 8,
          zIndex: 1,
          color: '#5f6472',
        }}
      >
        <X size={16} />
      </IconButton>

      <Stack direction="row" spacing={2} alignItems="center">
        {/* Circular progress indicator */}
        <Box
          sx={{
            position: 'relative',
            width: 56,
            height: 56,
            flexShrink: 0,
          }}
        >
          <CircularProgress
            variant="determinate"
            value={100}
            size={56}
            thickness={4}
            sx={{ color: '#ffd7bd', position: 'absolute', inset: 0 }}
          />
          <CircularProgress
            variant="determinate"
            value={progressValue}
            size={56}
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
              gap: 0.25,
            }}
          >
            <Typography sx={{ fontWeight: 700, lineHeight: 1, fontSize: 15 }}>
              {`${completedSteps}/${TOTAL_STEPS}`}
            </Typography>
            <Typography
              sx={{ lineHeight: 1.1, fontSize: 10, color: '#667085' }}
            >
              Steps
            </Typography>
          </Box>
        </Box>

        {/* Text content */}
        <Stack spacing={0.25} sx={{ minWidth: 0, flex: 1 }}>
          {isActive ? (
            <>
              <Typography
                sx={{ fontSize: 15, fontWeight: 600, lineHeight: 1.4 }}
              >
                You Successfully Configured Your{' '}
                <Box component="span" sx={{ fontWeight: 700 }}>
                  {resolvedDisplayName}
                </Box>{' '}
                Gateway & it&apos;s{' '}
                <Box
                  component="span"
                  sx={{ color: '#0b6e40', fontWeight: 700 }}
                >
                  Active
                </Box>{' '}
                now
              </Typography>
              <Typography
                sx={{ fontSize: 13, color: '#667085', lineHeight: 1.4 }}
              >
                <Box component="span" sx={{ fontWeight: 600 }}>
                  Next:{' '}
                </Box>
                Create LLM Provider and deploy on your new gateway.
              </Typography>
            </>
          ) : (
            <>
              <Typography
                sx={{ fontSize: 15, fontWeight: 600, lineHeight: 1.4 }}
              >
                You Successfully Created Your{' '}
                <Box component="span" sx={{ fontWeight: 700 }}>
                  {resolvedDisplayName}
                </Box>{' '}
                Gateway
              </Typography>
              <Typography
                sx={{ fontSize: 13, color: '#667085', lineHeight: 1.4 }}
              >
                <Box component="span" sx={{ fontWeight: 600 }}>
                  Next:{' '}
                </Box>
                Configure your {resolvedDisplayName.toLowerCase()} Gateway
              </Typography>
            </>
          )}
        </Stack>
        {isActive && (
          <Tooltip
            title={
              ''
            }
            disableHoverListener={!isProviderQuotaReached}
          >
            <Box component="span" sx={{ flexShrink: 0 }}>
              <Button
                variant="contained"
                disabled={isProviderQuotaReached}
                onClick={() => {
                  onFinish?.();
                  navigate(newProviderPath);
                }}
                endIcon={<ArrowRight size={16} />}
                sx={{ mr: 3, right: 25 }}
              >
                Continue
              </Button>
            </Box>
          </Tooltip>
        )}
      </Stack>
    </Card>
  );
}
