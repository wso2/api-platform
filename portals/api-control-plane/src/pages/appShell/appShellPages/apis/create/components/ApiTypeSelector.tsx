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

import { alpha, Box, Chip, Form, Stack, Tooltip, Typography } from '@wso2/oxygen-ui';
import { CircleCheck } from '@wso2/oxygen-ui-icons-react';
import { useState } from 'react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import { selectableCardSx } from '@/theme/receipes';
import type { ApiType } from '../types';
import { API_TYPES } from '../uiConfig';

const CARD_WIDTH = 264;

const messages = defineMessages({
  comingSoon: {
    id: 'api.create.ApiTypeSelector.badge.comingSoon',
    defaultMessage: 'Coming soon',
    description: 'Badge on an API type that is planned but not released yet.',
  },
  comingSoonHint: {
    id: 'api.create.ApiTypeSelector.tooltip.comingSoon',
    defaultMessage: 'Not available yet.',
    description: 'Tooltip explaining why an unreleased API type card cannot be clicked.',
  },
  available: {
    id: 'api.create.ApiTypeSelector.badge.available',
    defaultMessage: 'Available',
    description: 'Badge on an API type that can be selected now.',
  },
  groupLabel: {
    id: 'api.create.ApiTypeSelector.groupLabel',
    defaultMessage: 'API type',
    description: 'Accessible name for the group of API type cards. Noun, not a command.',
  },
  selected: {
    id: 'api.create.ApiTypeSelector.badge.selected',
    defaultMessage: 'Selected',
    description: 'Accessible label for the check mark on the chosen card.',
  },
  subtitle: {
    id: 'api.create.ApiTypeSelector.subtitle',
    defaultMessage: 'Choose how the gateway should expose your backend.',
    description: 'Supporting line under the API type selector heading.',
  },
  title: {
    id: 'api.create.ApiTypeSelector.title',
    defaultMessage: 'What kind of API are you exposing?',
    description: 'Heading above the grid of API type cards.',
  },
});

export type ApiTypeSelectorProps = {
  /**
   * Called with the whole catalog entry, not just its key; so the caller can
   * render the picked type's title and icon without looking it back up.
   */
  onChange: (apiType: ApiType) => void;
  /**
   * Key of the selected type. Pass it to drive the selection from the parent;
   * omit it and the component keeps its own selection, starting empty.
   */
  value?: string;
};

/**
 * A row of compact cards, one per API type the platform will ever offer. Each
 * card carries the type's mark and name; its longer description hangs off the
 * card as a tooltip rather than taking up room on the surface.
 *
 * Types that are not released yet stay on screen behind a "Coming soon" chip
 * rather than being hidden, the set is meant to show the full shape of the
 * product. `enabled` on the shared {@link API_TYPES} catalog is the single
 * source for what is pickable, so releasing WebSocket (say) is one flag flip in
 * `uiConfig.tsx` and needs no change here.
 */
export const ApiTypeSelector = ({ onChange, value }: ApiTypeSelectorProps) => {
  const intl = useIntl();
  const [internalKey, setInternalKey] = useState<string | undefined>(undefined);

  // Controlled the moment the caller passes `value`; self-managed otherwise.
  const selectedKey = value ?? internalKey;

  const handleSelect = (apiType: ApiType) => {
    setInternalKey(apiType.key);
    onChange(apiType);
  };

  return (
    // The compact, left-aligned grid is deliberately bounded to three columns.
    <Stack
      spacing={3}
      sx={{
        maxWidth: CARD_WIDTH * 3 + 32,
        width: '100%',
      }}
    >
      <Box
        aria-label={intl.formatMessage(messages.groupLabel)}
        role="group"
        sx={{ display: 'flex', flexWrap: 'wrap', gap: 2, justifyContent: 'flex-start' }}
      >
        {API_TYPES.map((apiType) => {
          const disabled = !apiType.enabled;
          const selected = !disabled && apiType.key === selectedKey;

          return (
            <Tooltip
              key={apiType.key}
              title={disabled ? intl.formatMessage(messages.comingSoonHint) : ''}
            >
              {/* The disabled card takes no pointer events (see below), so the
                  tooltip needs a plain element of its own to hang off. */}
              <Box sx={{ display: 'flex' }}>
                <Form.CardButton
                  alignItems="flex-start"
                  aria-disabled={disabled || undefined}
                  disabled={disabled}
                  onClick={disabled ? undefined : () => handleSelect(apiType)}
                  selected={selected}
                  sx={(theme) => ({
                    ...selectableCardSx(theme, { disabled, selected }),
                    height: 142,
                    justifyContent: 'space-between',
                    p: 2,
                    width: CARD_WIDTH,
                    '& > *': { width: '100%' },
                    ...(selected && {
                      backgroundColor: alpha(theme.palette.primary.main, 0.1),
                    }),
                    ...(disabled && {
                      borderColor: alpha(theme.palette.text.primary, 0.55),
                      cursor: 'default',
                      pointerEvents: 'none',
                    }),
                  })}
                  tabIndex={disabled ? -1 : undefined}
                  variant="outlined"
                >
                  <Stack
                    direction="row"
                    sx={{ alignItems: 'flex-start', justifyContent: 'space-between' }}
                  >
                    {apiType.icon}
                    <Chip
                      color={disabled ? 'default' : 'primary'}
                      label={
                        <FormattedMessage
                          {...(disabled ? messages.comingSoon : messages.available)}
                        />
                      }
                      size="small"
                      variant="outlined"
                    />
                  </Stack>
                  <Stack spacing={0.25} sx={{ textAlign: 'left' }}>
                    <Stack direction="row" spacing={0.5} sx={{ alignItems: 'center' }}>
                      <Form.Body sx={{ fontWeight: 700 }}>
                        <FormattedMessage {...apiType.title} />
                      </Form.Body>
                      {selected ? (
                        <Box
                          aria-label={intl.formatMessage(messages.selected)}
                          role="img"
                          sx={{ color: 'primary.main', display: 'flex' }}
                        >
                          <CircleCheck size={16} />
                        </Box>
                      ) : null}
                    </Stack>
                    <Typography color="text.secondary" variant="caption">
                      <FormattedMessage {...apiType.description} />
                    </Typography>
                  </Stack>
                </Form.CardButton>
              </Box>
            </Tooltip>
          );
        })}
      </Box>
    </Stack>
  );
};
