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

import type { ReactNode } from 'react';
import { Box, Chip, Form, Grid, Stack, Typography } from '@wso2/oxygen-ui';
import { Network, Sparkles, Zap } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage, useIntl, type MessageDescriptor } from 'react-intl';

import { selectableCardSx } from '@/theme/receipes';
import type { GatewayFunctionality } from '../utils/gatewayDisplay';

const messages = defineMessages({
  aiTitle: {
    id: 'gateways.create.type.ai.title',
    defaultMessage: 'AI Gateway',
  },
  beta: {
    id: 'gateways.create.type.badge.beta',
    defaultMessage: 'Beta',
    description: 'Badge on a gateway type that is released but not yet generally available.',
  },
  eventTitle: {
    id: 'gateways.create.type.event.title',
    defaultMessage: 'Event Gateway',
  },
  groupLabel: {
    id: 'gateways.create.type.groupLabel',
    defaultMessage: 'Gateway type',
    description: 'Accessible name for the row of gateway type cards. Noun, not a command.',
  },
  regularTitle: {
    id: 'gateways.create.type.regular.title',
    defaultMessage: 'API Gateway',
  },
});

type GatewayTypeOption = {
  /** Shown as a small badge; the type is still pickable. */
  beta?: boolean;
  icon: ReactNode;
  title: MessageDescriptor;
  value: GatewayFunctionality;
};

/**
 * Every `functionalityType` the spec allows, in the order they are offered.
 * `Record` rather than a bare array so a new value in the spec fails typecheck
 * here instead of silently dropping a card off the row.
 */
const GATEWAY_TYPES: Record<GatewayFunctionality, GatewayTypeOption> = {
  regular: {
    icon: <Network size={22} />,
    title: messages.regularTitle,
    value: 'regular',
  },
  ai: {
    icon: <Sparkles size={22} />,
    title: messages.aiTitle,
    value: 'ai',
  },
  event: {
    beta: true,
    icon: <Zap size={22} />,
    title: messages.eventTitle,
    value: 'event',
  },
};

const GATEWAY_TYPE_ORDER: GatewayFunctionality[] = ['regular', 'ai', 'event'];

/** Edge of the tinted square each type's icon sits in. */
const ICON_TILE_SIZE = 40;

export type GatewayTypeSelectorProps = {
  onChange: (value: GatewayFunctionality) => void;
  value: GatewayFunctionality;
};

/**
 * The kind of traffic the gateway will serve, as a single row of cards at the
 * top of the create form.
 *
 * It leads the form because it is the one answer the others are written
 * against; the type is immutable after creation, while every field below it
 * can be edited later. Each card carries only an icon and a name.
 */
export const GatewayTypeSelector = ({ onChange, value }: GatewayTypeSelectorProps) => {
  const intl = useIntl();

  return (
    <Grid
      aria-label={intl.formatMessage(messages.groupLabel)}
      container
      role="radiogroup"
      spacing={2}
    >
      {GATEWAY_TYPE_ORDER.map((key) => {
        const option = GATEWAY_TYPES[key];
        const selected = option.value === value;

        return (
          <Grid key={option.value} size={{ sm: 4, xs: 12 }}>
            <Form.CardButton
              alignItems="center"
              aria-checked={selected}
              onClick={() => onChange(option.value)}
              role="radio"
              selected={selected}
              sx={(theme) => ({
                ...selectableCardSx(theme, { selected }),
                height: '100%',
                p: 1.5,
                width: '100%',
              })}
              variant="outlined"
            >
              <Stack direction="row" spacing={1.5} sx={{ alignItems: 'center', width: '100%' }}>
                <Box
                  sx={{
                    alignItems: 'center',
                    bgcolor: 'action.hover',
                    borderRadius: 2,
                    color: 'text.primary',
                    display: 'flex',
                    flexShrink: 0,
                    height: ICON_TILE_SIZE,
                    justifyContent: 'center',
                    width: ICON_TILE_SIZE,
                  }}
                >
                  {option.icon}
                </Box>
                <Typography noWrap sx={{ fontWeight: 600 }} variant="body1">
                  <FormattedMessage {...option.title} />
                </Typography>
                {option.beta ? (
                  <Chip
                    label={<FormattedMessage {...messages.beta} />}
                    size="small"
                    sx={{ ml: 'auto' }}
                  />
                ) : null}
              </Stack>
            </Form.CardButton>
          </Grid>
        );
      })}
    </Grid>
  );
};
