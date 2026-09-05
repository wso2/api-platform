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

import { alpha, Box, Button, CircularProgress, Stack, Typography, useTheme } from '@wso2/oxygen-ui';
import { useEffect, useState } from 'react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import { ErrorState } from '@/components/StateViews';

const messages = defineMessages({
  back: {
    id: 'api.create.ApiCreationProgress.action.back',
    defaultMessage: 'Back to configuration',
    description: 'Returns the user to the form they submitted, after creation failed.',
  },
  failedBody: {
    id: 'api.create.ApiCreationProgress.failed.body',
    defaultMessage: 'Nothing was created. Check the details you entered and try again.',
  },
  failedTitle: {
    id: 'api.create.ApiCreationProgress.failed.title',
    defaultMessage: 'We could not create this API proxy',
  },
  percent: {
    id: 'api.create.ApiCreationProgress.progress.value',
    defaultMessage: '{value}%',
    description: 'Share of the creation flow completed so far.',
  },
  progressLabel: {
    id: 'api.create.ApiCreationProgress.progress.label',
    defaultMessage: 'API proxy creation progress',
    description: 'Accessible name for the circular progress indicator.',
  },
  retry: {
    id: 'api.create.ApiCreationProgress.action.retry',
    defaultMessage: 'Try again',
  },
  stageCreating: {
    id: 'api.create.ApiCreationProgress.stage.creating',
    defaultMessage: 'Creating API Proxy',
    description: 'Status shown while the platform is creating the API proxy.',
  },
  stageDone: {
    id: 'api.create.ApiCreationProgress.stage.done',
    defaultMessage: 'API Proxy created. Taking you there…',
  },
  stageFinalizing: {
    id: 'api.create.ApiCreationProgress.stage.finalizing',
    defaultMessage: 'Finalizing setup',
  },
  stageValidating: {
    id: 'api.create.ApiCreationProgress.stage.validating',
    defaultMessage: 'Validating configuration',
  },
  subtitle: {
    id: 'api.create.ApiCreationProgress.subtitle',
    defaultMessage: '“{name}” will be ready in a moment.',
    description: '{name} is the display name the user gave the API. Never translated.',
  },
  title: {
    id: 'api.create.ApiCreationProgress.title',
    defaultMessage: 'We are in the process of creating your API Proxy',
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
};

/**
 * How the percentage behaves.
 *
 * Creation is a single request, so there are no real milestones to report —
 * the bar eases toward a ceiling it never reaches on its own and only snaps to
 * 100 when the server actually answers. That keeps the number honest in the
 * one direction that matters: it never claims "done" before it is.
 */
const PROGRESS_TICK_MS = 350;
const PROGRESS_EASING = 0.12;
const PENDING_CEILING = 92;
/** Long enough for "100%" to register before the page changes under the user. */
const COMPLETION_HOLD_MS = 800;

/** Stage copy by percentage reached, ordered. */
const STAGES = [
  { label: messages.stageValidating, until: 35 },
  { label: messages.stageCreating, until: 70 },
  { label: messages.stageFinalizing, until: 100 },
] as const;

const stageFor = (percent: number) =>
  (STAGES.find((stage) => percent < stage.until) ?? STAGES[STAGES.length - 1]).label;

/** Diameter of the progress ring, in px. Shared by the ring and its track. */
const RING_SIZE = 56;
const RING_THICKNESS = 3;

/** Geometry of the illustration, in its own viewBox units. */
const ART = {
  height: 120,
  hexRadius: 46,
  width: 380,
} as const;

/**
 * Rendered width of the illustration, in px. The drawing scales to it from the
 * viewBox, so this is the only knob for how large it appears on screen.
 */
const ART_DISPLAY_WIDTH = 720;

/**
 * Tooth counts of the two gears, which double as their turn durations in
 * seconds. Meshing gears turn at a ratio set by their tooth counts, so one
 * second per tooth keeps the pair honest — and is slow enough not to nag.
 */
const GEAR_TEETH_LARGE = 9;
const GEAR_TEETH_SMALL = 8;

/**
 * Points of a regular pointy-top hexagon, as an SVG `points` string.
 */
const hexagonPoints = (cx: number, cy: number, radius: number): string =>
  Array.from({ length: 6 }, (_, index) => {
    // Start at -90° so a vertex sits at the top, then step a sixth of a turn.
    const angle = -Math.PI / 2 + (Math.PI / 3) * index;
    return `${(cx + radius * Math.cos(angle)).toFixed(2)},${(cy + radius * Math.sin(angle)).toFixed(
      2,
    )}`;
  }).join(' ');

/**
 * A gear outline as a single path, with the axle bore cut out of it.
 *
 * Generated rather than hand-written: a gear is the same four points repeated
 * per tooth, and computing them keeps the tooth count a parameter instead of
 * sixty hand-tuned coordinates. The bore is a second subpath, so the caller
 * renders this with `fillRule="evenodd"` to punch it through.
 */
const gearPath = (
  cx: number,
  cy: number,
  tip: number,
  root: number,
  bore: number,
  teeth: number,
): string => {
  const step = (Math.PI * 2) / teeth;
  const toothHalf = step * 0.19;
  const flank = step * 0.07;
  const at = (radius: number, angle: number) =>
    `${(cx + radius * Math.cos(angle)).toFixed(2)},${(cy + radius * Math.sin(angle)).toFixed(2)}`;

  const outline = Array.from({ length: teeth }, (_, index) => {
    const angle = index * step;
    return [
      at(tip, angle - toothHalf),
      at(tip, angle + toothHalf),
      at(root, angle + toothHalf + flank),
      at(root, angle + step - toothHalf - flank),
    ].join(' L');
  }).join(' L');

  // The bore is drawn as two half-arcs — a circle has no single-command form.
  const boreCircle =
    `M${cx + bore},${cy} ` +
    `A${bore},${bore} 0 1,0 ${cx - bore},${cy} ` +
    `A${bore},${bore} 0 1,0 ${cx + bore},${cy} Z`;

  return `M${outline} Z ${boreCircle}`;
};

/**
 * The waiting illustration: a backend on one side, a client on the other, and
 * the proxy being assembled between them.
 *
 * Inline SVG rather than an asset because every colour is a palette token — it
 * follows the theme into dark mode, which a flat exported image cannot.
 */
const ApiProxyAssemblyArt = () => {
  const theme = useTheme();
  const centreX = ART.width / 2;
  const centreY = ART.height / 2;
  const outline = theme.palette.primary.main;
  const cog = theme.palette.primary.contrastText;
  // Pivots are stated in viewBox units rather than left to `fill-box`: a gear's
  // bounding box isn't centred on its axle for odd tooth counts, so the default
  // origin would make it wobble instead of spin.
  const largeGear = { cx: centreX + 6, cy: centreY - 6 };
  const smallGear = { cx: centreX - 14, cy: centreY + 14 };

  return (
    <Box
      aria-hidden
      component="svg"
      sx={{
        // The spin lives here rather than in a `style` prop, so the
        // reduced-motion rule below can actually override it.
        '& .gear--large': { animation: `apicpGearSpin ${GEAR_TEETH_LARGE}s linear infinite` },
        '& .gear--small': {
          // Meshed with the larger gear, so it has to turn the other way.
          animation: `apicpGearSpinReverse ${GEAR_TEETH_SMALL}s linear infinite`,
        },
        '@keyframes apicpGearSpin': {
          from: { transform: 'rotate(0deg)' },
          to: { transform: 'rotate(360deg)' },
        },
        '@keyframes apicpGearSpinReverse': {
          from: { transform: 'rotate(0deg)' },
          to: { transform: 'rotate(-360deg)' },
        },
        // Motion here is decorative; anyone who has asked for less gets the
        // same illustration, held still.
        '@media (prefers-reduced-motion: reduce)': {
          '& .gear--large, & .gear--small': { animation: 'none' },
        },
        display: 'block',
        maxWidth: '100%',
        width: ART_DISPLAY_WIDTH,
      }}
      viewBox={`0 0 ${ART.width} ${ART.height}`}
    >
      {/* Client, left: a diamond. */}
      <polygon
        fill="none"
        points={`36,${centreY - 20} 56,${centreY} 36,${centreY + 20} 16,${centreY}`}
        stroke={outline}
        strokeLinejoin="round"
        strokeWidth={2}
      />
      {/* Backend, right: a circle. */}
      <circle
        cx={ART.width - 36}
        cy={centreY}
        fill="none"
        r={20}
        stroke={outline}
        strokeWidth={2}
      />

      {/* Traffic on both hops, as dotted runs through a junction node. */}
      {[
        { from: 64, node: 118, to: 112 },
        { from: ART.width - 112, node: ART.width - 118, to: ART.width - 64 },
      ].map((hop) => (
        <g key={hop.node}>
          <line
            stroke={alpha(theme.palette.primary.main, 0.5)}
            strokeDasharray="1 9"
            strokeLinecap="round"
            strokeWidth={3}
            x1={hop.from}
            x2={hop.to}
            y1={centreY}
            y2={centreY}
          />
          <circle cx={hop.node} cy={centreY} fill="none" r={4} stroke={outline} strokeWidth={2} />
        </g>
      ))}

      {/* The proxy itself, with its two gears turning against each other. */}
      <polygon fill={outline} points={hexagonPoints(centreX, centreY, ART.hexRadius)} />
      <path
        className="gear--large"
        d={gearPath(largeGear.cx, largeGear.cy, 17, 13, 7, GEAR_TEETH_LARGE)}
        fill={cog}
        fillRule="evenodd"
        style={{ transformOrigin: `${largeGear.cx}px ${largeGear.cy}px` }}
      />
      <circle cx={largeGear.cx} cy={largeGear.cy} fill={cog} r={3.5} />
      <path
        className="gear--small"
        d={gearPath(smallGear.cx, smallGear.cy, 10, 7.5, 3.5, GEAR_TEETH_SMALL)}
        fill={cog}
        fillRule="evenodd"
        style={{ transformOrigin: `${smallGear.cx}px ${smallGear.cy}px` }}
      />
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
  displayName,
  onBack,
  onComplete,
  onRetry,
  status,
}: ApiCreationProgressProps) => {
  const intl = useIntl();
  const [percent, setPercent] = useState(0);

  useEffect(() => {
    if (status !== 'creating') return undefined;

    // A retry re-enters `creating` with the ring left near the ceiling from the
    // failed attempt, which would open the second try at "Finalizing setup".
    // Already 0 on the first pass, so this only ever undoes a stale climb.
    setPercent(0);

    const timer = window.setInterval(() => {
      setPercent((current) =>
        Math.min(
          PENDING_CEILING,
          current + Math.max(1, (PENDING_CEILING - current) * PROGRESS_EASING),
        ),
      );
    }, PROGRESS_TICK_MS);

    return () => window.clearInterval(timer);
  }, [status]);

  useEffect(() => {
    if (status !== 'created') return undefined;

    setPercent(100);
    const timer = window.setTimeout(onComplete, COMPLETION_HOLD_MS);

    return () => window.clearTimeout(timer);
  }, [onComplete, status]);

  const rounded = Math.round(percent);

  return (
    <Stack spacing={4} sx={{ alignItems: 'center', py: 8, textAlign: 'center', width: '100%' }}>
      <Stack spacing={1} sx={{ alignItems: 'center', maxWidth: 'sm' }}>
        <Typography sx={{ fontWeight: 700 }} variant="h1">
          <FormattedMessage {...messages.title} />
        </Typography>
        {displayName && (
          <Typography color="text.secondary" variant="body1">
            <FormattedMessage {...messages.subtitle} values={{ name: displayName }} />
          </Typography>
        )}
      </Stack>

      <ApiProxyAssemblyArt />

      {status === 'failed' ? (
        <Stack spacing={2} sx={{ maxWidth: 'sm', width: '100%' }}>
          <ErrorState
            message={intl.formatMessage(messages.failedBody)}
            title={intl.formatMessage(messages.failedTitle)}
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
        <Stack spacing={2} sx={{ alignItems: 'center' }}>
          <Box sx={{ display: 'inline-flex', position: 'relative' }}>
            {/* Track behind the arc, so the remaining share stays readable. */}
            <CircularProgress
              size={RING_SIZE}
              sx={{ color: 'divider' }}
              thickness={RING_THICKNESS}
              value={100}
              variant="determinate"
            />
            <CircularProgress
              aria-label={intl.formatMessage(messages.progressLabel)}
              size={RING_SIZE}
              sx={{ color: 'primary.main', left: 0, position: 'absolute' }}
              thickness={RING_THICKNESS}
              value={rounded}
              variant="determinate"
            />
            <Box
              sx={{
                alignItems: 'center',
                display: 'flex',
                inset: 0,
                justifyContent: 'center',
                position: 'absolute',
              }}
            >
              <Typography color="text.secondary" variant="caption">
                <FormattedMessage {...messages.percent} values={{ value: rounded }} />
              </Typography>
            </Box>
          </Box>

          <Typography
            aria-live="polite"
            color="primary.main"
            sx={{ fontWeight: 600 }}
            variant="body1"
          >
            <FormattedMessage
              {...(status === 'created' ? messages.stageDone : stageFor(percent))}
            />
          </Typography>
        </Stack>
      )}
    </Stack>
  );
};
