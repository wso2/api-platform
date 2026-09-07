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

import {
  alpha,
  Step,
  StepButton,
  StepConnector,
  stepConnectorClasses,
  stepIconClasses,
  StepLabel,
  stepLabelClasses,
  Stepper,
  styled,
} from '@wso2/oxygen-ui';
import { defineMessages, FormattedMessage } from 'react-intl';

const messages = defineMessages({
  apiType: {
    id: 'api.create.ApiCreationSteps.step.apiType',
    defaultMessage: 'API type',
    description: 'Progress step where the user picks REST, GraphQL and so on. Noun, not a command.',
  },
  configure: {
    id: 'api.create.ApiCreationSteps.step.configure',
    defaultMessage: 'Configure',
    description: 'Progress step where the user fills in the API name, version and context.',
  },
  source: {
    id: 'api.create.ApiCreationSteps.step.source',
    defaultMessage: 'Source',
    description: 'Progress step where the user supplies a contract or a backend endpoint.',
  },
});

/**
 * The steps of the API creation flow, in order.
 *
 * Held here rather than in `uiConfig.tsx` so the bar stays self-contained: a
 * page only has to say which step it is on, and this file owns both the order
 * and the copy.
 */
const API_CREATION_STEPS = [
  { key: 'apiType', label: messages.apiType },
  { key: 'source', label: messages.source },
  { key: 'configure', label: messages.configure },
] as const;

/** Union of the step keys, so a caller cannot name a step that doesn't exist. */
export type ApiCreationStepKey = (typeof API_CREATION_STEPS)[number]['key'];

export type ApiCreationStepsProps = {
  /** The step the user is on now. Everything before it renders as completed. */
  activeStep: ApiCreationStepKey;
  /**
   * Called when a *completed* step is clicked. Omit it and the bar is a pure
   * progress display; the current and upcoming steps are never clickable
   * either way, since the flow can only move forward through its own actions.
   */
  onStepClick?: (step: ApiCreationStepKey) => void;
};

/** Step circle diameter; connector offset is derived from this. */
const STEP_ICON_SIZE = 32;

/**
 * Connector line between steps. Tints completed lines with primary color.
 */
const ProgressConnector = styled(StepConnector)(({ theme }) => ({
  [`& .${stepConnectorClasses.line}`]: {
    borderColor: theme.palette.divider,
  },
  [`&.${stepConnectorClasses.active} .${stepConnectorClasses.line}, &.${stepConnectorClasses.completed} .${stepConnectorClasses.line}`]:
    {
      borderColor: theme.palette.primary.main,
    },
  top: STEP_ICON_SIZE / 2,
}));

/**
 * Horizontal progress bar for the API creation flow: a numbered circle per
 * step, filled once reached, with a check mark on the ones already done.
 */
export const ApiCreationSteps = ({ activeStep, onStepClick }: ApiCreationStepsProps) => {
  const activeIndex = API_CREATION_STEPS.findIndex((step) => step.key === activeStep);

  return (
    <Stepper
      activeStep={activeIndex}
      connector={<ProgressConnector />}
      sx={(theme) => ({
        // The circle is an SVG, so its box is driven by `fontSize`, not width.
        [`& .${stepIconClasses.root}`]: { fontSize: STEP_ICON_SIZE },
        // Soft primary-tinted ring around the step being worked on. The
        // completed and current circles are both filled `primary.main`, so the
        // halo plus the bolder label below are what separate "here" from
        // "done". `50%` is the circle's geometry, not a radius choice.
        [`& .${stepIconClasses.root}.${stepIconClasses.active}`]: {
          borderRadius: '50%',
          boxShadow: `0 0 0 ${theme.spacing(0.5)} ${alpha(theme.palette.primary.main, 0.16)}`,
        },
        [`& .${stepLabelClasses.label}`]: { typography: 'body1' },
        [`& .${stepLabelClasses.label}.${stepLabelClasses.active}`]: {
          color: 'text.primary',
          fontWeight: theme.typography.fontWeightBold,
        },
        maxWidth: theme.breakpoints.values.sm,
        mx: 'auto',
        width: '100%',
      })}
    >
      {API_CREATION_STEPS.map((step, index) => {
        const completed = index < activeIndex;
        const label = <FormattedMessage {...step.label} />;

        return (
          <Step completed={completed} key={step.key}>
            {completed && onStepClick ? (
              <StepButton onClick={() => onStepClick(step.key)}>{label}</StepButton>
            ) : (
              <StepLabel>{label}</StepLabel>
            )}
          </Step>
        );
      })}
    </Stepper>
  );
};
