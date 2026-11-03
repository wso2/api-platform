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

import { alpha, Box, Chip, ListItemButton, ListSubheader, Typography, type Theme } from '@wso2/oxygen-ui';
import { ChevronDown, ChevronRight } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, useIntl } from 'react-intl';

import { hairline } from '@/theme/receipes';

const messages = defineMessages({
  toggleGroup: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentGroupHeader.toggleGroup',
    defaultMessage: '{type} ({count})',
    description:
      'Accessible name of a document-type group header that expands or collapses the group.',
  },
});

/**
 * `action.selected` is nearly invisible on the dark acrylic surface, so the selected item also
 * gets a primary border. Every item has a transparent border of the same width to avoid layout shift.
 */
export const documentItemSx = (theme: Theme) =>
  ({
    border: hairline(theme),
    borderColor: 'transparent',
    mb: 0.5,
    '&.Mui-selected, &.Mui-selected:hover': {
      backgroundColor: alpha(theme.palette.primary.main, 0.08),
      borderColor: 'primary.main',
    },
  }) as const;

type DocumentGroupHeaderProps = {
  count: number;
  /** The id of the collapsible region this header toggles. */
  controlsId: string;
  first: boolean;
  label: string;
  onToggle: () => void;
  open: boolean;
};

/** A document-type group's header: the type, how many documents it holds, and a toggle for them. */
export function DocumentGroupHeader({ controlsId, count, first, label, onToggle, open }: DocumentGroupHeaderProps) {
  const intl = useIntl();
  return (
    <ListSubheader
      disableGutters
      disableSticky
      sx={{
        bgcolor: 'transparent',
        lineHeight: 'normal',
        mb: open ? 1 : 0,
        mt: first ? 0.5 : 1.5,
      }}
    >
      {/* The header is the toggle: a real button, so it is reachable by keyboard. */}
      <ListItemButton
        aria-controls={controlsId}
        aria-expanded={open}
        aria-label={intl.formatMessage(messages.toggleGroup, { count, type: label })}
        dense
        onClick={onToggle}
        sx={{ gap: 1, px: 1, py: 0.5 }}
      >
        <Box sx={{ color: 'text.secondary', display: 'flex' }}>
          {open ? <ChevronDown size={16} /> : <ChevronRight size={16} />}
        </Box>
        <Typography
          color="text.secondary"
          noWrap
          sx={{ flex: 1, fontWeight: 600, minWidth: 0, textTransform: 'none' }}
          variant="body2"
        >
          {label}
        </Typography>
        <Chip label={count} size="small" sx={{ typography: 'caption' }} />
      </ListItemButton>
    </ListSubheader>
  );
}
