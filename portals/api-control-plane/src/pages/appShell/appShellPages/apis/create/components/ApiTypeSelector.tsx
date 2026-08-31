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

import { Box, Chip, Form, Grid, Stack, Tooltip, Typography } from '@wso2/oxygen-ui';
import { CircleCheck } from '@wso2/oxygen-ui-icons-react';
import { useState } from 'react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import { selectableCardSx } from '@/theme/receipes';
import type { ApiType } from '../types';
import { API_TYPES } from '../uiConfig';

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
    defaultMessage:
      'This decides how the gateway exposes your backend. Only REST is available today.',
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
   * Called with the whole catalog entry — not just its key — so the caller can
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
 * A grid of cards, one per API type the platform will ever offer.
 *
 * Types that are not released yet stay on screen behind a "Coming soon" chip
 * rather than being hidden, the grid is meant to show the full shape of the
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
    // Centered, max-width layout; 3 columns at `md` (wraps 3 + 2).
    <Stack
      spacing={3}
      sx={{
        maxWidth: (theme) => theme.breakpoints.values.md,
        mx: 'auto',
        px: { md: 4, xs: 2 },
        width: '100%',
      }}
    >
      <Grid
        aria-label={intl.formatMessage(messages.groupLabel)}
        container
        role="group"
        spacing={2}
        sx={{ justifyContent: 'center' }}
      >
        {API_TYPES.map((apiType) => {
          const disabled = !apiType.enabled;
          const selected = !disabled && apiType.key === selectedKey;

          return (
            <Grid key={apiType.key} size={{ xs: 12, sm: 6, md: 4 }}>
              <Tooltip title={disabled ? intl.formatMessage(messages.comingSoonHint) : ''}>
                {/* A disabled CardButton fires no pointer events, so the
                    tooltip needs a plain element of its own to hang off. */}
                <Box sx={{ height: '100%' }}>
                  <Form.CardButton
                    alignItems="center"
                    disabled={disabled}
                    onClick={() => handleSelect(apiType)}
                    selected={selected}
                    sx={(theme) => ({
                      ...selectableCardSx(theme, { disabled, selected }),
                      height: '100%',
                      p: 2,
                      width: '100%',
                    })}
                    variant="outlined"
                  >
                    <Stack
                      spacing={1}
                      sx={{
                        alignItems: 'center',
                        height: '100%',
                        textAlign: 'center',
                        width: '100%',
                      }}
                    >
                      {apiType.icon}
                      <Stack direction="row" spacing={0.5} sx={{ alignItems: 'center' }}>
                        <Typography sx={{ fontWeight: 600 }} variant="body2">
                          <FormattedMessage {...apiType.title} />
                        </Typography>
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
                      {disabled ? (
                        <Chip
                          label={<FormattedMessage {...messages.comingSoon} />}
                          size="small"
                          sx={{ mt: 'auto' }}
                        />
                      ) : null}
                    </Stack>
                  </Form.CardButton>
                </Box>
              </Tooltip>
            </Grid>
          );
        })}
      </Grid>
    </Stack>
  );
};
