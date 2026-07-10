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

import React, { useEffect, useMemo, useState } from 'react';
import {
  Box,
  Card,
  CircularProgress,
  IconButton,
  Stack,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import {
  ArrowRight,
  Package,
  Rocket,
  ShieldCheck,
  X,
} from '@wso2/oxygen-ui-icons-react';
import QuickStartCompletedIcon from '../../../../../assets/images/quickStart/QuickStartCompletedIcon';
import {
  getLLMProviderAPIKeys,
  getLLMProviderDeployments,
} from '../../../../../apis/llmProviderApis';
import { useAppShell } from '../../../../../contexts/AppShellContext';
import { useAIEntity } from '../../../../../contexts/AIEntitiesContext';
import { PLATFORM_API_BASE_URL } from '../../../../../config.env';
import type { LLMProvider } from '../../../../../utils/types';

export type LLLMStepBannerStepId =
  | 'add-guardrails'
  | 'deploy-to-gateway'
  | 'consume';

type StepIconComponent = React.ComponentType<{ size?: string | number }>;

type StepDefinition = {
  id: LLLMStepBannerStepId;
  title: string;
  icon: StepIconComponent;
  completed: boolean;
};

type LLLMStepBannerProps = {
  providerName?: string;
  onStepClick?: (stepId: LLLMStepBannerStepId) => void;
  refreshTrigger?: string | number;
  tooltipStepId?: LLLMStepBannerStepId;
  tooltipMessage?: string;
};

const DEFAULT_TOTAL_STEPS = 4;

export default function LLLMStepBanner({
  providerName,
  onStepClick,
  refreshTrigger,
  tooltipStepId,
  tooltipMessage,
}: LLLMStepBannerProps) {
  const [isVisible, setIsVisible] = useState(true);
  const [stepCompletion, setStepCompletion] = useState({
    hasDeployments: false,
    hasConsumptions: false,
  });
  const { entityType, entity } = useAIEntity();
  const { currentOrganization } = useAppShell();
  const selectedProvider =
    entityType === 'llm-provider' ? (entity as LLMProvider | null) : null;
  const selectedProviderId = selectedProvider?.id ?? '';
  const organizationId = currentOrganization?.uuid ?? '';
  const hasGuardrails =
    (selectedProvider?.policies?.filter(
      (p) => !(p.name === 'llm-cost' && p.version === 'v1')
    ).length ?? 0) > 0;
  const resolvedProviderName = providerName?.trim() || 'LLM Provider';

  useEffect(() => {
    if (!selectedProviderId || !organizationId) {
      setStepCompletion({
        hasDeployments: false,
        hasConsumptions: false,
      });
      return;
    }

    let isCancelled = false;

    const fetchStepCompletion = async () => {
      const [deploymentsResult, apiKeysResult] =
        await Promise.allSettled([
          getLLMProviderDeployments(
            selectedProviderId,
            organizationId,
            PLATFORM_API_BASE_URL
          ),
          getLLMProviderAPIKeys(selectedProviderId, organizationId),
        ]);

      if (isCancelled) {
        return;
      }

      setStepCompletion({
        hasDeployments:
          deploymentsResult.status === 'fulfilled'
            ? (deploymentsResult.value.list?.length ?? 0) > 0
            : false,
        hasConsumptions:
          apiKeysResult.status === 'fulfilled'
            ? (apiKeysResult.value.list?.length ?? 0) > 0
            : false,
      });
    };

    fetchStepCompletion().catch(() => {
      if (!isCancelled) {
        setStepCompletion({
          hasDeployments: false,
          hasConsumptions: false,
        });
      }
    });

    return () => {
      isCancelled = true;
    };
  }, [selectedProviderId, organizationId, PLATFORM_API_BASE_URL, refreshTrigger]);

  const steps = useMemo<StepDefinition[]>(
    () => [
      {
        id: 'add-guardrails',
        title: 'Add Guardrails',
        icon: ShieldCheck,
        completed: hasGuardrails,
      },
      {
        id: 'deploy-to-gateway',
        title: 'Deploy to Gateway',
        icon: Rocket,
        completed: stepCompletion.hasDeployments,
      },
      {
        id: 'consume',
        title: 'Consume LLM Provider',
        icon: Package,
        completed: stepCompletion.hasConsumptions,
      },
    ],
    [hasGuardrails, stepCompletion]
  );

  const totalSteps = DEFAULT_TOTAL_STEPS;
  const completedSteps =
    1 + steps.filter((step) => step.completed).length;
  const progressValue =
    totalSteps > 0 ? (completedSteps / totalSteps) * 100 : 0;

  if (!isVisible) {
    return null;
  }

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
        aria-label="Close LLM setup steps banner"
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

      <Stack
        direction={{ xs: 'column', md: 'row' }}
        spacing={{ xs: 2, md: 2.5 }}
        alignItems={{ xs: 'flex-start', md: 'center' }}
      >
        <Box
          sx={{
            position: 'relative',
            width: 64,
            height: 64,
            flexShrink: 0,
          }}
        >
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
              {`${completedSteps}/${totalSteps}`}
            </Typography>
            <Typography
              sx={{ lineHeight: 1.1, fontSize: 10, color: '#667085' }}
            >
              Steps
            </Typography>
          </Box>
        </Box>

        <Stack spacing={0.6} sx={{ minWidth: 0, flex: 1 }}>
          <Box
            sx={{
              gap: 0.1,
              display: 'flex',
              flexDirection: 'column',
            }}
          >
            <Typography
              sx={{
                fontSize: 16,
                fontWeight: 500,
                lineHeight: 1.35,
              }}
            >
              <Box component="span">Successfully Created </Box>
              <Box component="span" sx={{ fontWeight: 700 }}>
                {resolvedProviderName}
              </Box>
              <Box component="span"> Provider</Box>
            </Typography>
            <Typography
              sx={{
                mt: 0.35,
                fontSize: 12,
                color: '#667085',
                lineHeight: 1.4,
              }}
            >
              Click each step to continue configuring and exposing your LLM
              provider.
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
            {steps.map((step) => {
              const Icon = step.icon;
              const isCompleted = step.completed;
              const isTooltipStep = tooltipStepId === step.id;
              const hoverTooltip =
                step.id === 'consume' && !stepCompletion.hasDeployments
                  ? 'Deploy to a gateway first before consuming the LLM provider'
                  : '';
              const tooltipTitle = isTooltipStep && tooltipMessage ? tooltipMessage : hoverTooltip;
              const tooltipOpen = isTooltipStep && !!tooltipMessage ? true : undefined;

              return (
                <Tooltip
                  key={step.id}
                  title={tooltipTitle}
                  placement="top"
                  open={tooltipOpen}
                  arrow
                >
                <Box
                  component="button"
                  type="button"
                  onClick={() => onStepClick?.(step.id)}
                  sx={{
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
                    transition:
                      'border-color 0.2s ease, box-shadow 0.2s ease, transform 0.2s ease',
                    '&:hover': {
                      borderColor: '#ff6701',
                      boxShadow: '0 4px 12px rgba(19, 93, 138, 0.12)',
                      transform: 'translateY(-1px)',
                    },
                  }}
                >
                  {isCompleted ? (
                    <QuickStartCompletedIcon size={16} />
                  ) : (
                    <Icon size={16} />
                  )}
                  <Typography
                    sx={{
                      fontSize: 14,
                      fontWeight: 500,
                      whiteSpace: 'nowrap',
                    }}
                  >
                    {step.title}
                  </Typography>
                  <ArrowRight size={16} color='#9b9b9b'/>
                </Box>
                </Tooltip>
              );
            })}
          </Box>
        </Stack>
      </Stack>
    </Card>
  );
}
