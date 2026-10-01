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

import { ToggleButton, ToggleButtonGroup } from '@wso2/oxygen-ui';
import { Eye, Pencil } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import { segmentedSwitchSx } from '@/theme/receipes';
import type { PublicationVersionTone } from './PublicationVersionCard';

const messages = defineMessages({
  groupLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.PublicationVersionToggle.groupLabel',
    defaultMessage: 'Version to show',
    description: 'Accessible name for the Draft / Published buttons that choose which version of the listing is on screen.',
  },
  draft: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.PublicationVersionToggle.draft',
    defaultMessage: 'Draft',
    description: 'Button showing the editable working copy of the listing. Noun.',
  },
  published: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.PublicationVersionToggle.published',
    defaultMessage: 'Published',
    description: 'Button showing the live, read-only version of the listing. Adjective.',
  },
});

type PublicationVersionToggleProps = {
  /** Off while an action is running. */
  disabled?: boolean;
  onChange: (version: PublicationVersionTone) => void;
  /** There is no published version to show until the API is live on the portal. */
  publishedAvailable: boolean;
  value: PublicationVersionTone;
};

/**
 * Moves the page between the draft being edited and the published version it
 * came from. The app's segmented pill: a quiet track with the current choice
 * raised in it, so the control reads as a direction — Draft to Published and
 * back. The raised segment's label takes the colour of the banner and border
 * it switches on: primary for the draft, success for the published version.
 */
export function PublicationVersionToggle({
  disabled,
  onChange,
  publishedAvailable,
  value,
}: PublicationVersionToggleProps) {
  const intl = useIntl();

  return (
    <ToggleButtonGroup
      aria-label={intl.formatMessage(messages.groupLabel)}
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
