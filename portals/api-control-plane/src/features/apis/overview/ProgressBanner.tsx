import {
  Box,
  ButtonBase,
  LinearProgress,
  Stack,
  Typography,
} from '@wso2/oxygen-ui';
import {
  Check,
  FilePlus2,
  FlaskConical,
  Globe,
  Rocket,
  type LucideIcon,
} from '@wso2/oxygen-ui-icons-react';
import { useNavigate, useParams } from 'react-router-dom';

import { routes } from '../../../routes/paths';
import type { ApiDetail } from '../../../types/domain';

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
 *   Deploy   → live on a gateway, or staged/published
 *   Test     → staged (PENDING) or published (ACTIVE)
 *   Publish  → published to the dev portal (ACTIVE)
 * The steps double as the navigation the old Deploy/Test/Manage buttons gave.
 */
export function ProgressBanner({
  detail,
  deployed,
}: {
  detail: ApiDetail;
  deployed: boolean;
}) {
  const { orgHandle = '', projectHandler = '', apiHandler = '' } = useParams();
  const navigate = useNavigate();

  const published = detail.status === 'ACTIVE';
  const staged = detail.status === 'PENDING';
  const deployComplete = deployed || staged || published;
  const testComplete = staged || published;

  const steps: ProgressStep[] = [
    { key: 'create', label: 'Create', Icon: FilePlus2, complete: true },
    {
      key: 'deploy',
      label: 'Deploy',
      Icon: Rocket,
      complete: deployComplete,
      onClick: () =>
        navigate(routes.apiDeploy(orgHandle, projectHandler, apiHandler)),
    },
    {
      key: 'test',
      label: 'Test',
      Icon: FlaskConical,
      complete: testComplete,
      onClick: () =>
        navigate(routes.apiTest(orgHandle, projectHandler, apiHandler)),
    },
    {
      key: 'publish',
      label: 'Publish to Devportal',
      Icon: Globe,
      complete: published,
      onClick: () =>
        navigate(routes.apiManage(orgHandle, projectHandler, apiHandler)),
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
    <Box
      sx={{
        bgcolor: 'background.paper',
        border: '1px solid',
        borderColor: 'divider',
        borderRadius: 1,
        mb: 3,
        p: 2,
      }}
    >
      <Stack
        alignItems="center"
        direction="row"
        justifyContent="space-between"
        sx={{ mb: 1 }}
      >
        <Typography sx={{ fontWeight: 600 }} variant="h6">
          Track your progress here
        </Typography>
        <Typography color="text.secondary" variant="caption">
          {completedCount} of {steps.length} completed
        </Typography>
      </Stack>
      <LinearProgress
        color={percent === 100 ? 'success' : 'primary'}
        sx={{ borderRadius: 1, height: 6, mb: 2 }}
        value={percent}
        variant="determinate"
      />
      <Stack
        alignItems="center"
        direction="row"
        spacing={1}
        sx={{ flexWrap: 'wrap', rowGap: 1 }}
      >
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
    </Box>
  );
}

/** Line between two steps: solid green when both done, dashed when leading into
 * the current step, grey otherwise — mirrors the choreo stepper connectors. */
function StepConnector({
  fromState,
  toState,
}: {
  fromState: StepState;
  toState: StepState;
}) {
  if (fromState === 'complete' && toState === 'complete') {
    return (
      <Box
        sx={{ bgcolor: 'success.main', borderRadius: 1, height: 2, width: 24 }}
      />
    );
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
  return (
    <Box sx={{ bgcolor: 'divider', borderRadius: 1, height: 2, width: 24 }} />
  );
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
        borderColor: complete
          ? 'success.main'
          : active
            ? 'primary.main'
            : 'divider',
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
          color: complete
            ? 'common.white'
            : active
              ? 'primary.main'
              : 'text.primary',
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
