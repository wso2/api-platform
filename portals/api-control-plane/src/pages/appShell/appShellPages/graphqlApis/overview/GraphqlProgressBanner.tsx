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

import { Box, ButtonBase, Divider, Stack, Typography } from '@wso2/oxygen-ui';
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

import { routes } from '@/routes/paths';

const messages = defineMessages({
  progress: {
    id: 'apiControlPlane.pages.appShell.appShellPages.graphqlApis.overview.GraphqlProgressBanner.progress',
    defaultMessage: '{completed} of {total} completed',
    description:
      'Counter beside the lifecycle steps, e.g. "2 of 4 completed". Counts the steps of getting an API live, not APIs.',
  },
  next: {
    id: 'apiControlPlane.pages.appShell.appShellPages.graphqlApis.overview.GraphqlProgressBanner.next',
    defaultMessage: 'Next: {step}',
    description: 'The next incomplete lifecycle step shown beside the progress counter.',
  },
  stepCreate: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.ProgressBanner.step.create',
    defaultMessage: 'Create',
    description: 'First step of the API progress stepper — the API record exists. A stage name, not a button command.',
  },
  stepDeploy: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.ProgressBanner.step.deploy',
    defaultMessage: 'Deploy',
    description: 'Second step of the API progress stepper — the API runs on a gateway. A stage name, not a button command.',
  },
  stepTest: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.ProgressBanner.step.test',
    defaultMessage: 'Test',
    description: 'Third step of the API progress stepper — the API has been called from the test console. A stage name, not a button command.',
  },
  stepPublish: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.ProgressBanner.step.publish',
    defaultMessage: 'Publish to Devportal',
    description: 'Fourth step of the API progress stepper — the API is listed in the developer portal.',
  },
});

type StepState = 'complete' | 'active' | 'upcoming';

type ProgressStep = {
  key: string;
  label: string;
  Icon: LucideIcon;
  state: StepState;
  onClick?: () => void;
};

/**
 * Slimmed sibling of `apis/overview/ProgressBanner.tsx` for GraphQL APIs — see
 * that file for the visual design this mirrors, including the "N of 4
 * completed | Next: X" summary beside the steps. Only two steps track real
 * completion (Create is always done; Deploy reflects whether the API is live
 * on any gateway): `GraphQLAPIDetail` carries no `lifeCycleStatus`, so there is
 * no honest signal for "tested" or "published" the way REST's stepper has.
 * Test is offered as a next action once deployed, never marked complete.
 * Publish stays disabled unconditionally — it leads to a `ComingSoon` stub
 * with nothing behind it yet, so unlike Test it never becomes a real next
 * action a user should be steered toward. Because of this, the summary caps
 * out at "2 of 4 completed" once deployed and never advances further — an
 * honest reflection of there being no more real signal to report, not a bug.
 */
export function GraphqlProgressBanner({ deployed }: { deployed: boolean }) {
  const { orgHandle = '', projectHandler = '', graphqlApiHandler = '' } = useParams();
  const navigate = useNavigate();
  const intl = useIntl();

  const steps: ProgressStep[] = [
    {
      key: 'create',
      label: intl.formatMessage(messages.stepCreate),
      Icon: FilePlus2,
      state: 'complete',
    },
    {
      key: 'deploy',
      label: intl.formatMessage(messages.stepDeploy),
      Icon: Rocket,
      state: deployed ? 'complete' : 'active',
      onClick: () => navigate(routes.graphqlApiDeploy(orgHandle, projectHandler, graphqlApiHandler)),
    },
    {
      key: 'test',
      label: intl.formatMessage(messages.stepTest),
      Icon: FlaskConical,
      state: deployed ? 'active' : 'upcoming',
      onClick: deployed
        ? () =>
            navigate(routes.graphqlApiTestConsole(orgHandle, projectHandler, graphqlApiHandler))
        : undefined,
    },
    {
      key: 'publish',
      label: intl.formatMessage(messages.stepPublish),
      Icon: Globe,
      // Always disabled, regardless of deploy state — unlike Test, this isn't
      // a real next action yet (see `GraphqlPublishPage`, a `ComingSoon`
      // stub), so it never becomes clickable the way REST's own equivalent
      // step is. Matches the disabled treatment `apis/overview/ProgressBanner`
      // gives an upcoming step.
      state: 'upcoming',
    },
  ];

  const completedCount = steps.filter((step) => step.state === 'complete').length;
  // The first not-yet-complete step is the current, actionable one — same
  // derivation as `apis/overview/ProgressBanner`, applied to this banner's own
  // per-step `state` instead of a `complete` boolean.
  const activeIndex = steps.findIndex((step) => step.state !== 'complete');

  return (
    <Box
      sx={{
        borderTop: '1px solid',
        borderColor: 'divider',
        px: { sm: 4, xs: 2 },
        py: 2,
      }}
    >
      <Stack
        alignItems="center"
        direction={{ md: 'row', xs: 'column' }}
        justifyContent="space-between"
        spacing={2}
      >
        <Stack alignItems="center" direction="row" spacing={1} sx={{ flexWrap: 'wrap', rowGap: 1 }}>
          {steps.map((step, index) => (
            <Stack alignItems="center" direction="row" key={step.key} spacing={1}>
              <StepPill step={step} />
              {index < steps.length - 1 && <StepConnector fromState={step.state} />}
            </Stack>
          ))}
        </Stack>
        <Stack
          alignItems="center"
          direction="row"
          divider={<Divider flexItem orientation="vertical" />}
          spacing={1.5}
          sx={{ flexShrink: 0 }}
        >
          <Typography color="text.secondary" variant="body2">
            <FormattedMessage
              {...messages.progress}
              values={{ completed: completedCount, total: steps.length }}
            />
          </Typography>
          {activeIndex >= 0 && (
            <Typography color="text.secondary" variant="body2">
              <FormattedMessage
                {...messages.next}
                values={{
                  step: (
                    <Box component="span" sx={{ color: 'text.primary', fontWeight: 700 }}>
                      {steps[activeIndex].label}
                    </Box>
                  ),
                }}
              />
            </Typography>
          )}
        </Stack>
      </Stack>
    </Box>
  );
}

function StepConnector({ fromState }: { fromState: StepState }) {
  if (fromState === 'complete') {
    return <Box sx={{ bgcolor: 'success.main', borderRadius: 1, height: 2, width: 24 }} />;
  }
  return <Box sx={{ bgcolor: 'divider', borderRadius: 1, height: 2, width: 24 }} />;
}

function StepPill({ step }: { step: ProgressStep }) {
  const { Icon, label, onClick, state } = step;
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
        minWidth: 0,
        justifyContent: 'flex-start',
        borderColor: complete ? 'success.main' : active ? 'primary.main' : 'divider',
        borderRadius: 5,
        gap: 1,
        opacity: upcoming ? 0.6 : 1,
        px: 1.25,
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
        {complete ? <Check size={18} /> : <Icon size={16} />}
      </Box>
      <Typography sx={{ fontWeight: 600 }} variant="body2">
        {label}
      </Typography>
    </ButtonBase>
  );
}
