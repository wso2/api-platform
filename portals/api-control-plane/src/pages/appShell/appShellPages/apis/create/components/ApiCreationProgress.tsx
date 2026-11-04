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

import { alpha, Box, Button, LinearProgress, Stack, Typography, useTheme } from '@wso2/oxygen-ui';
import { Check, Database, Monitor } from '@wso2/oxygen-ui-icons-react';
import { useEffect, useState } from 'react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import { ErrorState } from '@/components/StateViews';

const messages = defineMessages({
  back: {
    id: 'api.create.ApiCreationProgress.action.back',
    defaultMessage: 'Back to configuration',
    description: 'Returns the user to the form they submitted, after creation failed.',
  },
  backend: {
    id: 'api.create.ApiCreationProgress.node.backend',
    defaultMessage: 'Backend',
  },
  client: {
    id: 'api.create.ApiCreationProgress.node.client',
    defaultMessage: 'Client',
  },
  failedBody: {
    id: 'api.create.ApiCreationProgress.failed.body',
    defaultMessage: 'Nothing was created. Check the details you entered and try again.',
  },
  failedTitle: {
    id: 'api.create.ApiCreationProgress.failed.title',
    defaultMessage:
      'We could not create this {kind, select, graphql {GraphQL API} other {API proxy}}',
  },
  percent: {
    id: 'api.create.ApiCreationProgress.progress.value',
    defaultMessage: '{value}%',
    description: 'Share of the creation flow completed so far.',
  },
  progressLabel: {
    id: 'api.create.ApiCreationProgress.progress.label',
    defaultMessage: '{kind, select, graphql {GraphQL API} other {API proxy}} creation progress',
    description: 'Accessible name for the progress bar.',
  },
  retry: {
    id: 'api.create.ApiCreationProgress.action.retry',
    defaultMessage: 'Try again',
  },
  stageDone: {
    id: 'api.create.ApiCreationProgress.stage.done',
    defaultMessage:
      '{kind, select, graphql {GraphQL API} other {API Proxy}} created. Taking you there…',
  },
  stepDeploying: {
    id: 'api.create.ApiCreationProgress.step.deploying',
    defaultMessage: 'Creating {kind, select, graphql {GraphQL API} other {API Proxy}}',
  },
  stepStateActive: {
    id: 'api.create.ApiCreationProgress.step.state.active',
    defaultMessage: 'In progress',
    description: 'Screen-reader text for the step currently running.',
  },
  stepStateDone: {
    id: 'api.create.ApiCreationProgress.step.state.done',
    defaultMessage: 'Completed',
    description: 'Screen-reader text for a finished step.',
  },
  stepStatePending: {
    id: 'api.create.ApiCreationProgress.step.state.pending',
    defaultMessage: 'Pending',
    description: 'Screen-reader text for a step not yet started.',
  },
  stepValidating: {
    id: 'api.create.ApiCreationProgress.step.validating',
    defaultMessage: 'Validating API definition',
  },
  title: {
    id: 'api.create.ApiCreationProgress.title',
    defaultMessage:
      'We are in the process of creating your {kind, select, graphql {GraphQL API} other {API Proxy}}',
  },
  titleWithName: {
    id: 'api.create.ApiCreationProgress.titleWithName',
    defaultMessage:
      'We are in the process of creating your {name} {kind, select, graphql {GraphQL API} other {API Proxy}}',
    description: '{name} is the display name the user gave the API. Never translated.',
  },
});

/** Where the flow is, as far as this screen is concerned. */
export type ApiCreationProgressStatus = 'creating' | 'created' | 'failed';

export type ApiCreationProgressProps = {
  /** The name the user gave the API, shown back to them while they wait. */
  displayName?: string;
  /** Leaves this screen for the form that produced the request. */
  onBack: () => void;
  /**
   * Fired once, shortly after `status` turns `created` — the caller navigates
   * to the new API from here. Hold it stable (`useCallback`), or the hold timer
   * restarts on every render.
   */
  onComplete: () => void;
  /** Re-issues the same create request. */
  onRetry: () => void;
  status: ApiCreationProgressStatus;
  /** What is being created, named in the screen's copy. Defaults to REST's API proxy. */
  apiKind?: 'graphql' | 'rest';
};

/**
 * How the percentage behaves.
 *
 * Creation is a single request, so only the last step reflects real work. The
 * first step is paced by the clock — short, so a quick create is not
 * held back — and fill the bar to `TIMED_SHARE`. The last step then waits on
 * the server: the bar eases toward a ceiling it never reaches on its own and
 * snaps to 100 only when the server actually answers, so the number never
 * claims "done" before it is.
 */
const TICK_MS = 100;
const STEP_MS = 450;
const STEP_COUNT = 2;
const TIMED_STEPS = STEP_COUNT - 1;
const TIMED_MS = STEP_MS * TIMED_STEPS;
const TIMED_SHARE = (100 / STEP_COUNT) * TIMED_STEPS;
const PENDING_CEILING = 96;
const PENDING_EASING = 0.05;
/** Long enough for "100%" to register before the page changes under the user. */
const COMPLETION_HOLD_MS = 800;

const STEPS = [messages.stepValidating, messages.stepDeploying] as const;

/** The step in progress at a given percentage; the last one once the timed steps are through. */
const activeStepFor = (percent: number) =>
  Math.min(STEP_COUNT - 1, Math.floor(percent / (100 / STEP_COUNT)));

/** Read by assistive tech, invisible on screen. */
const visuallyHidden = {
  border: 0,
  clip: 'rect(0 0 0 0)',
  height: '1px',
  margin: '-1px',
  overflow: 'hidden',
  padding: 0,
  position: 'absolute',
  whiteSpace: 'nowrap',
  width: '1px',
} as const;

const NODE_SIZE = 64;
const CORE_SIZE = 150;
const CORE_SIZE_NARROW = 110;

/**
 * The waiting illustration: a client and a backend with the proxy between
 * them, packets travelling both hops.
 *
 * Built from theme tokens rather than a fixed palette, so it follows the theme
 * into dark mode. Its motion is decorative and stops for anyone who has asked
 * for less.
 */
const ApiProxyFlowArt = ({ complete }: { complete: boolean }) => {
  const theme = useTheme();
  const intl = useIntl();
  const accent = theme.palette.primary.main;
  const track = theme.palette.divider;

  const node = (icon: typeof Monitor, label: string, done: boolean) => {
    const Icon = icon;
    return (
      <Stack spacing={1.5} sx={{ alignItems: 'center', flexShrink: 0 }}>
        <Box
          sx={{
            alignItems: 'center',
            bgcolor: done ? alpha(accent, 0.16) : 'background.paper',
            border: `1.5px solid ${accent}`,
            borderRadius: '16px',
            color: accent,
            display: 'flex',
            height: NODE_SIZE,
            justifyContent: 'center',
            width: NODE_SIZE,
          }}
        >
          <Icon size={26} strokeWidth={1.6} />
        </Box>
        <Typography color="text.secondary" variant="body2">
          {label}
        </Typography>
      </Stack>
    );
  };

  const line = (delays: number[]) => (
    <Box
      sx={{
        // Lifted by the node label's height so the line meets the boxes' midpoint.
        background: complete
          ? accent
          : `repeating-linear-gradient(90deg, ${track} 0 6px, transparent 6px 12px)`,
        flex: 1,
        height: 2,
        mb: '25px',
        minWidth: 8,
        mx: { sm: '14px', xs: '6px' },
        overflow: 'hidden',
        position: 'relative',
      }}
    >
      {!complete &&
        delays.map((delay) => (
          <Box
            key={delay}
            sx={{
              animation: `apicpPacket 1.6s linear ${delay}s infinite`,
              bgcolor: accent,
              borderRadius: '50%',
              boxShadow: `0 0 10px ${accent}`,
              height: 6,
              left: 0,
              position: 'absolute',
              top: -2,
              width: 6,
            }}
          />
        ))}
    </Box>
  );

  return (
    <Box
      aria-hidden
      sx={{
        '@keyframes apicpPacket': {
          '0%': { left: '0%', opacity: 0 },
          '10%': { opacity: 1 },
          '90%': { opacity: 1 },
          '100%': { left: '100%', opacity: 0 },
        },
        '@keyframes apicpSpin': { to: { transform: 'rotate(360deg)' } },
        '@keyframes apicpSpinReverse': { to: { transform: 'rotate(-360deg)' } },
        '@keyframes apicpGlow': {
          '0%, 100%': { opacity: 0.5, transform: 'scale(0.92)' },
          '50%': { opacity: 1, transform: 'scale(1.05)' },
        },
        '@keyframes apicpBreathe': {
          '0%, 100%': { transform: 'scale(1)' },
          '50%': { transform: 'scale(1.06)' },
        },
        '@media (prefers-reduced-motion: reduce)': {
          '& *': { animation: 'none !important' },
        },
        alignItems: 'center',
        display: 'flex',
        justifyContent: 'center',
        maxWidth: 620,
        width: '100%',
      }}
    >
      {node(Monitor, intl.formatMessage(messages.client), true)}
      {line([0, 0.53, 1.06])}

      <Box
        sx={{
          alignItems: 'center',
          display: 'flex',
          flexShrink: 0,
          height: { sm: CORE_SIZE, xs: CORE_SIZE_NARROW },
          justifyContent: 'center',
          mb: '25px',
          position: 'relative',
          width: { sm: CORE_SIZE, xs: CORE_SIZE_NARROW },
        }}
      >
        <Box
          sx={{
            animation: 'apicpGlow 2.4s ease-in-out infinite',
            background: `radial-gradient(circle, ${alpha(accent, 0.25)} 0%, transparent 65%)`,
            borderRadius: '50%',
            inset: -20,
            position: 'absolute',
          }}
        />
        <Box
          sx={{
            animation: 'apicpSpin 18s linear infinite',
            border: `1.5px dashed ${track}`,
            borderRadius: '50%',
            inset: 0,
            position: 'absolute',
          }}
        />
        <Box
          sx={{
            animation: 'apicpSpin 1.6s linear infinite',
            border: '2px solid transparent',
            borderRadius: '50%',
            borderRightColor: alpha(accent, 0.35),
            borderTopColor: accent,
            inset: 14,
            opacity: complete ? 0.4 : 1,
            position: 'absolute',
          }}
        />
        <Box
          sx={{
            animation: 'apicpSpinReverse 2.4s linear infinite',
            border: '1.5px solid transparent',
            borderBottomColor: alpha(accent, 0.6),
            borderRadius: '50%',
            inset: 28,
            opacity: complete ? 0.4 : 1,
            position: 'absolute',
          }}
        />
        <Box
          sx={{
            alignItems: 'center',
            animation: 'apicpBreathe 2.4s ease-in-out infinite',
            bgcolor: accent,
            borderRadius: '18px',
            boxShadow: `0 10px 30px ${alpha(accent, 0.4)}`,
            color: theme.palette.primary.contrastText,
            display: 'flex',
            height: NODE_SIZE,
            justifyContent: 'center',
            position: 'relative',
            width: NODE_SIZE,
          }}
        >
          <svg
            fill="none"
            height={30}
            stroke="currentColor"
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={1.8}
            viewBox="0 0 24 24"
            width={30}
          >
            <path d="M12 3 20 7.5v9L12 21 4 16.5v-9z" />
            <path d="M9 10.5 7 12l2 1.5M15 10.5l2 1.5-2 1.5M13 9.5l-2 5" />
          </svg>
        </Box>
      </Box>

      {line([0.8, 1.33, 1.86])}
      {node(Database, intl.formatMessage(messages.backend), complete)}
    </Box>
  );
};

/**
 * Full-page wait state for API creation: what is happening, how far along it
 * is, and — when it fails — the two ways out.
 *
 * It owns the percentage but not the outcome. The caller drives `status` from
 * the mutation and decides where "created" leads, so this stays a display.
 */
export const ApiCreationProgress = ({
  apiKind = 'rest',
  displayName,
  onBack,
  onComplete,
  onRetry,
  status,
}: ApiCreationProgressProps) => {
  const intl = useIntl();
  const theme = useTheme();
  const [percent, setPercent] = useState(0);

  useEffect(() => {
    if (status !== 'creating') return undefined;

    // A retry re-enters `creating` with the bar left near the ceiling from the
    // failed attempt, which would open the second try at the last step. Already
    // 0 on the first pass, so this only ever undoes a stale climb.
    setPercent(0);
    const startedAt = Date.now();

    const timer = window.setInterval(() => {
      const elapsed = Date.now() - startedAt;
      if (elapsed < TIMED_MS) {
        setPercent((TIMED_SHARE * elapsed) / TIMED_MS);
        return;
      }
      // Past the paced steps the bar only moves because the server is still working.
      setPercent((current) =>
        Math.min(
          PENDING_CEILING,
          Math.max(TIMED_SHARE, current) +
            Math.max(0.2, (PENDING_CEILING - Math.max(TIMED_SHARE, current)) * PENDING_EASING),
        ),
      );
    }, TICK_MS);

    return () => window.clearInterval(timer);
  }, [status]);

  useEffect(() => {
    if (status !== 'created') return undefined;

    setPercent(100);
    const timer = window.setTimeout(onComplete, COMPLETION_HOLD_MS);

    return () => window.clearTimeout(timer);
  }, [onComplete, status]);

  const rounded = Math.round(percent);
  const complete = status === 'created';
  const activeStep = activeStepFor(percent);
  const accent = theme.palette.primary.main;

  return (
    <Stack spacing={6} sx={{ alignItems: 'center', py: 6, textAlign: 'center', width: '100%' }}>
      <Stack sx={{ alignItems: 'center', maxWidth: 'md' }}>
        <Typography sx={{ fontWeight: 700 }} variant="h2">
          <FormattedMessage
            {...(displayName ? messages.titleWithName : messages.title)}
            values={displayName ? { kind: apiKind, name: displayName } : { kind: apiKind }}
          />
        </Typography>
      </Stack>

      <ApiProxyFlowArt complete={complete} />

      {status === 'failed' ? (
        <Stack spacing={2} sx={{ maxWidth: 'sm', width: '100%' }}>
          <ErrorState
            message={intl.formatMessage(messages.failedBody)}
            title={intl.formatMessage(messages.failedTitle, { kind: apiKind })}
          />
          <Stack direction="row" spacing={1} sx={{ justifyContent: 'center' }}>
            <Button onClick={onBack} variant="outlined">
              <FormattedMessage {...messages.back} />
            </Button>
            <Button onClick={onRetry} variant="contained">
              <FormattedMessage {...messages.retry} />
            </Button>
          </Stack>
        </Stack>
      ) : (
        <Stack spacing={2} sx={{ maxWidth: 360, width: '100%' }}>
          <LinearProgress
            aria-label={intl.formatMessage(messages.progressLabel, { kind: apiKind })}
            sx={{
              '& .MuiLinearProgress-bar': {
                background: `linear-gradient(90deg, ${accent}, ${alpha(accent, 0.7)})`,
                borderRadius: 3,
                transition: 'transform 0.3s ease-out',
              },
              borderRadius: 3,
              height: 6,
            }}
            value={rounded}
            variant="determinate"
          />
          <Stack direction="row" sx={{ justifyContent: 'space-between' }}>
            <Typography
              aria-live="polite"
              color="primary.main"
              sx={{ fontWeight: 500 }}
              variant="body2"
            >
              {complete ? (
                <FormattedMessage {...messages.stageDone} values={{ kind: apiKind }} />
              ) : (
                <FormattedMessage {...STEPS[activeStep]} values={{ kind: apiKind }} />
              )}
            </Typography>
            <Typography
              color="text.secondary"
              sx={{ fontVariantNumeric: 'tabular-nums' }}
              variant="body2"
            >
              <FormattedMessage {...messages.percent} values={{ value: rounded }} />
            </Typography>
          </Stack>

          <Stack
            component="ul"
            spacing={1.25}
            sx={{ listStyle: 'none', m: 0, p: 0, textAlign: 'left' }}
          >
            {STEPS.map((step, index) => {
              const done = complete || index < activeStep;
              const active = !complete && index === activeStep;
              return (
                <Stack
                  component="li"
                  direction="row"
                  aria-current={active ? 'step' : undefined}
                  key={step.id}
                  spacing={1.25}
                  sx={{
                    alignItems: 'center',
                    color: done ? 'text.secondary' : active ? 'text.primary' : 'text.disabled',
                    transition: 'color 0.2s',
                  }}
                >
                  <Box
                    sx={{
                      '@keyframes apicpStepSpin': { to: { transform: 'rotate(360deg)' } },
                      '@media (prefers-reduced-motion: reduce)': { animation: 'none' },
                      alignItems: 'center',
                      animation: active ? 'apicpStepSpin 0.8s linear infinite' : 'none',
                      bgcolor: done ? accent : 'transparent',
                      border: `1.5px solid ${done || active ? accent : theme.palette.divider}`,
                      borderRadius: '50%',
                      borderTopColor: active ? 'transparent' : undefined,
                      color: theme.palette.primary.contrastText,
                      display: 'flex',
                      flexShrink: 0,
                      height: 16,
                      justifyContent: 'center',
                      width: 16,
                    }}
                  >
                    {done && <Check size={10} strokeWidth={3} />}
                  </Box>
                  <Typography color="inherit" variant="body2">
                    <FormattedMessage {...step} values={{ kind: apiKind }} />
                  </Typography>
                  <Box component="span" sx={visuallyHidden}>
                    <FormattedMessage
                      {...(done
                        ? messages.stepStateDone
                        : active
                          ? messages.stepStateActive
                          : messages.stepStatePending)}
                    />
                  </Box>
                </Stack>
              );
            })}
          </Stack>
        </Stack>
      )}
    </Stack>
  );
};
