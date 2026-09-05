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

import { Box, ButtonBase, Card, LinearProgress, Stack, Typography } from '@wso2/oxygen-ui';
import {
  Check,
  FilePlus2,
  FlaskConical,
  Globe,
  Rocket,
  type LucideIcon,
} from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';
import { useNavigate, useParams } from 'react-router-dom';

import type { RestApi } from '@/api/resources/restApis';
import { routes } from '@/routes/paths';

const messages = defineMessages({
  progress: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.ProgressBanner.progress',
    defaultMessage: '{completed} of {total} completed',
    description:
      'Counter beside the progress bar, e.g. "2 of 4 completed". Counts the steps of getting an API live, not APIs.',
  },
  stepCreate: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.ProgressBanner.step.create',
    defaultMessage: 'Create',
    description:
      'First step of the API progress stepper — the API record exists. A stage name, not a button command.',
  },
  stepDeploy: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.ProgressBanner.step.deploy',
    defaultMessage: 'Deploy',
    description:
      'Second step of the API progress stepper — the API runs on a gateway. A stage name, not a button command.',
  },
  stepPublish: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.ProgressBanner.step.publish',
    defaultMessage: 'Publish to API Portal',
    description:
      'Fourth step of the API progress stepper — the API is listed in the developer portal, which ships under the name "API Portal".',
  },
  stepTest: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.ProgressBanner.step.test',
    defaultMessage: 'Test',
    description:
      'Third step of the API progress stepper — the API has been called from the test console. A stage name, not a button command.',
  },
  title: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.ProgressBanner.title',
    defaultMessage: 'Track your progress here',
    description:
      'Heading of the banner that walks the user through creating, deploying, testing and publishing an API.',
  },
});

type StepState = 'complete' | 'active' | 'upcoming';

type ProgressStep = {
  key: string;
  label: string;
  Icon: LucideIcon;
  complete: boolean;
  onClick?: () => void;
};

/**
 * "Track your progress here" banner, ported from the legacy choreo console
 * life-cycle stepper. The stepper is the progress chart: each step fills in
 * (green icon + tick, green connector) as it completes, and a determinate bar
 * shows the overall percentage.
 *
 * Completion is derived from the API's real lifecycle, which is monotonic —
 * publishing implies the API was deployed and tested, staging implies deploy
 * + test — so earlier steps stay green once a later one is reached:
 *   Create   → always done (the API record exists)
 *   Deploy   → live on a gateway, or STAGED/PUBLISHED
 *   Test     → STAGED or PUBLISHED
 *   Publish  → PUBLISHED to the dev portal
 * The steps double as the navigation the old Deploy/Test/Manage buttons gave.
 */
export function ProgressBanner({ api, deployed }: { api: RestApi; deployed: boolean }) {
  const { orgHandle = '', projectHandler = '', apiHandler = '' } = useParams();
  const navigate = useNavigate();
  const intl = useIntl();

  const published = api.lifeCycleStatus === 'PUBLISHED';
  const staged = api.lifeCycleStatus === 'STAGED';
  const deployComplete = deployed || staged || published;
  const testComplete = staged || published;

  // Formatted here rather than held as descriptors: `label` is both the pill's
  // text and its `aria-label`, and the latter is a string-only prop.
  const steps: ProgressStep[] = [
    {
      key: 'create',
      label: intl.formatMessage(messages.stepCreate),
      Icon: FilePlus2,
      complete: true,
    },
    {
      key: 'deploy',
      label: intl.formatMessage(messages.stepDeploy),
      Icon: Rocket,
      complete: deployComplete,
      onClick: () => navigate(routes.apiDeploy(orgHandle, projectHandler, apiHandler)),
    },
    {
      key: 'test',
      label: intl.formatMessage(messages.stepTest),
      Icon: FlaskConical,
      complete: testComplete,
      onClick: () => navigate(routes.apiTestConsole(orgHandle, projectHandler, apiHandler)),
    },
    {
      key: 'publish',
      label: intl.formatMessage(messages.stepPublish),
      Icon: Globe,
      complete: published,
      onClick: () => navigate(routes.apiManageLifecycle(orgHandle, projectHandler, apiHandler)),
    },
  ];

  const completedCount = steps.filter((step) => step.complete).length;
  const percent = Math.round((completedCount / steps.length) * 100);
  // The first not-yet-complete step is the current, actionable one.
  const activeIndex = steps.findIndex((step) => !step.complete);

  const stateOf = (step: ProgressStep, index: number): StepState => {
    if (step.complete) return 'complete';
    return index === activeIndex ? 'active' : 'upcoming';
  };

  return (
    <Card
      sx={{
        p: 2,
      }}
    >
      <Stack alignItems="center" direction="row" justifyContent="space-between" sx={{ mb: 1 }}>
        <Typography sx={{ fontWeight: 600 }} variant="h6">
          <FormattedMessage {...messages.title} />
        </Typography>
        <Typography color="text.secondary" variant="caption">
          <FormattedMessage
            {...messages.progress}
            values={{ completed: completedCount, total: steps.length }}
          />
        </Typography>
      </Stack>
      <LinearProgress
        color={percent === 100 ? 'success' : 'primary'}
        sx={{ borderRadius: 1, height: 6, mb: 2 }}
        value={percent}
        variant="determinate"
      />
      <Stack alignItems="center" direction="row" spacing={1} sx={{ flexWrap: 'wrap', rowGap: 1 }}>
        {steps.map((step, index) => (
          <Stack alignItems="center" direction="row" key={step.key} spacing={1}>
            <StepPill state={stateOf(step, index)} step={step} />
            {index < steps.length - 1 && (
              <StepConnector
                fromState={stateOf(step, index)}
                toState={stateOf(steps[index + 1], index + 1)}
              />
            )}
          </Stack>
        ))}
      </Stack>
    </Card>
  );
}

/** Line between two steps: solid green when both done, dashed when leading into
 * the current step, grey otherwise — mirrors the choreo stepper connectors. */
function StepConnector({ fromState, toState }: { fromState: StepState; toState: StepState }) {
  if (fromState === 'complete' && toState === 'complete') {
    return <Box sx={{ bgcolor: 'success.main', borderRadius: 1, height: 2, width: 24 }} />;
  }
  if (fromState === 'complete' && toState === 'active') {
    return (
      <Box
        sx={{
          borderTop: '2px dashed',
          borderColor: 'primary.main',
          width: 24,
        }}
      />
    );
  }
  return <Box sx={{ bgcolor: 'divider', borderRadius: 1, height: 2, width: 24 }} />;
}

function StepPill({ state, step }: { state: StepState; step: ProgressStep }) {
  const { Icon, label, onClick } = step;
  const complete = state === 'complete';
  const active = state === 'active';
  const upcoming = state === 'upcoming';

  return (
    <ButtonBase
      aria-label={label}
      disabled={!onClick}
      onClick={onClick}
      sx={{
        border: '1px solid',
        minWidth: 120,
        justifyContent: 'flex-start',
        borderColor: complete ? 'success.main' : active ? 'primary.main' : 'divider',
        borderRadius: 5,
        gap: 1,
        opacity: upcoming ? 0.6 : 1,
        px: 1,
        py: 0.75,
        transition: 'opacity 0.15s',
        '&:hover': onClick ? { opacity: 0.75 } : undefined,
      }}
    >
      <Box
        sx={{
          alignItems: 'center',
          bgcolor: complete ? 'success.main' : 'action.hover',
          borderRadius: '50%',
          color: complete ? 'common.white' : active ? 'primary.main' : 'text.primary',
          display: 'flex',
          height: 32,
          justifyContent: 'center',
          width: 32,
        }}
      >
        <Icon size={16} />
      </Box>
      <Typography sx={{ fontWeight: 600 }} variant="body2">
        {label}
      </Typography>
      {complete && <Check color="var(--mui-palette-success-main)" size={16} />}
    </ButtonBase>
  );
}
