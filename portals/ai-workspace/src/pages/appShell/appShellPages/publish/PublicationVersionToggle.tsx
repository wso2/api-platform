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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import { defineMessages, FormattedMessage, useIntl } from 'react-intl';
import { ToggleButton, ToggleButtonGroup, toggleButtonGroupClasses, type Theme } from '@wso2/oxygen-ui';
import { Eye, Pencil } from '@wso2/oxygen-ui-icons-react';

import type { PublicationVersionTone } from './PublicationVersionCard';

const messages = defineMessages({
  ariaLabel: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.versionToggle.ariaLabel',
    defaultMessage: 'Version to show',
  },
  draft: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.versionToggle.draft',
    defaultMessage: 'Draft',
  },
  published: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.versionToggle.published',
    defaultMessage: 'Published',
  },
});

const PILL_RADIUS = '999px';
const SEGMENT_SHADOW = '0 1px 2px rgba(0, 0, 0, 0.12)';

/** A pill-shaped segmented switch; `accent` colours the active segment's label. */
const segmentedSwitchSx = (theme: Theme, accent: 'primary' | 'success') => {
  // `theme.vars` follows the active colour scheme; `theme.palette` is the default one.
  const palette = theme.vars?.palette ?? theme.palette;
  return {
    backgroundColor: palette.action.hover,
    borderRadius: PILL_RADIUS,
    gap: theme.spacing(0.5),
    padding: theme.spacing(0.5),
    [`& .${toggleButtonGroupClasses.grouped}`]: {
      border: 0,
      borderRadius: PILL_RADIUS,
      color: palette.text.secondary,
      fontWeight: theme.typography.fontWeightMedium,
      marginInline: 0,
      paddingInline: theme.spacing(2),
      textTransform: 'none',
      // Stronger than the track, or hovering an inactive segment lands on the
      // colour the track already is and reads as dead.
      '&:hover': { backgroundColor: palette.action.selected },
      // MUI outlines a disabled segment (and draws only its inner edge on the last
      // one), which breaks the pill's outline; the track already shows its extent.
      '&.Mui-disabled': { border: 0 },
      '&.Mui-selected': {
        backgroundColor: palette.background.paper,
        borderRadius: PILL_RADIUS,
        boxShadow: SEGMENT_SHADOW,
        color: palette[accent].main,
        fontWeight: theme.typography.fontWeightBold,
        ...theme.applyStyles('dark', { boxShadow: 'none' }),
        // Without this the active pill loses its fill on hover and the control flickers as the cursor crosses it.
        '&:hover': { backgroundColor: palette.background.paper },
      },
    },
  } as const;
};

type PublicationVersionToggleProps = {
  disabled?: boolean;
  onChange: (version: PublicationVersionTone) => void;
  /** There is no published version to show until the proxy is live on the hub. */
  publishedAvailable: boolean;
  value: PublicationVersionTone;
};

/** Switches between the draft and the published version, in the colour of the frame each one gets. */
export function PublicationVersionToggle({
  disabled,
  onChange,
  publishedAvailable,
  value,
}: PublicationVersionToggleProps) {
  const intl = useIntl();
  return (
    <ToggleButtonGroup
      aria-label={intl.formatMessage(messages.ariaLabel)}
      disabled={disabled}
      exclusive
      onChange={(_event, next: PublicationVersionTone | null) => {
        if (next !== null) onChange(next);
      }}
      size="small"
      sx={(theme) => ({
        ...segmentedSwitchSx(theme, value === 'published' ? 'success' : 'primary'),
        flexShrink: 0,
        mb: 1,
      })}
      value={value}
    >
      <ToggleButton sx={{ gap: 1 }} value="draft">
        <Pencil size={16} />
        <FormattedMessage {...messages.draft} />
      </ToggleButton>
      <ToggleButton disabled={disabled || !publishedAvailable} sx={{ gap: 1 }} value="published">
        <Eye size={16} />
        <FormattedMessage {...messages.published} />
      </ToggleButton>
    </ToggleButtonGroup>
  );
}
