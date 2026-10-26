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

import { alpha, Chip } from '@wso2/oxygen-ui';
import { Pencil, Trash2, TriangleAlert } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage, type MessageDescriptor } from 'react-intl';

const messages = defineMessages({
  inactive: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.StatusChip.inactive',
    defaultMessage: 'Inactive',
    description: 'Status pill on a selected plan whose subscription plan is inactive.',
  },
  deleted: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.StatusChip.deleted',
    defaultMessage: 'Deleted',
    description: 'Status pill on a selected plan or document whose source no longer exists.',
  },
  edited: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.StatusChip.edited',
    defaultMessage: 'Edited',
    description: 'Status pill on a published plan or document that was changed after it was last published.',
  },
});

/** The states a selected plan or document can show as a chip. */
export type Status = 'deleted' | 'inactive' | 'edited';

/** Palette colour of a status chip: warning for something to act on or check, error for something gone. */
type StatusTone = 'error' | 'warning';

const STATUS_CHIPS: Record<Status, { icon: typeof Trash2; label: MessageDescriptor; tone: StatusTone }> = {
  deleted: { icon: Trash2, label: messages.deleted, tone: 'error' },
  inactive: { icon: TriangleAlert, label: messages.inactive, tone: 'warning' },
  edited: { icon: Pencil, label: messages.edited, tone: 'warning' },
};

/** One state on a plan card or document row: a tinted pill with its icon, in the tone that says how much it matters. */
export function StatusChip({ status }: { status: Status }) {
  const { icon: Icon, label, tone } = STATUS_CHIPS[status];
  return (
    <Chip
      color={tone}
      icon={<Icon aria-hidden size={14} />}
      label={<FormattedMessage {...label} />}
      size="small"
      sx={(theme) => ({
        bgcolor: alpha(theme.palette[tone].main, 0.16),
        color: `${tone}.main`,
        flexShrink: 0,
        fontSize: 12,
        fontWeight: 600,
        letterSpacing: '0.01em',
        '& .MuiChip-icon': { color: 'inherit', ml: 1.25, mr: 0 },
        '& .MuiChip-label': { pl: 0.75, pr: 1.5 },
      })}
      variant="filled"
    />
  );
}
