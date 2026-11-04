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

import { alpha, Box, type Theme } from '@wso2/oxygen-ui';
import { Server } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, useIntl } from 'react-intl';

import { useScaleToWidth } from './useScaleToWidth';

const messages = defineMessages({
  backend: {
    id: 'apiControlPlane.components.ReadyToConnectIllustration.backend',
    defaultMessage: 'Backend',
    description: 'Title of the backend card in the "Ready to connect" preview. Noun.',
  },
});

/** Drawn at this size, then shrunk to fit narrower containers. */
const HUB_LABEL = '{ }';
const URL_SCHEME = 'https://';
const WIDTH = 468;
const HEIGHT = 168;

const resourceSx = (theme: Theme) => ({
  alignItems: 'center',
  bgcolor: alpha(theme.palette.text.primary, 0.1),
  border: 1,
  borderColor: 'divider',
  borderRadius: 0.5,
  boxSizing: 'border-box',
  display: 'flex',
  fontFamily: 'monospace',
  fontSize: 11,
  gap: 1,
  height: 32,
  px: 1.25,
});

const verbSx = (color: 'primary' | 'success') => (theme: Theme) => ({
  bgcolor: alpha(theme.palette[color].main, 0.18),
  borderRadius: 0.25,
  color: `${color}.main`,
  fontSize: 9.5,
  fontWeight: 600,
  px: 0.75,
  py: 0.25,
});

/**
 * "Ready to connect": three sample resources fan into an API node, which
 * hands off to a backend card waiting for its URL. Shown beside the backend
 * field of the start-from-scratch flow.
 *
 * Colours come from theme tokens so it follows light/dark, and it scales down to fit a narrower container. Purely decorative,
 * so it is hidden from assistive technology.
 */
export function ReadyToConnectIllustration() {
  const intl = useIntl();
  const { ref: frameRef, scale } = useScaleToWidth(WIDTH);
  const rows = [
    { method: 'GET', color: 'success', path: '/orders' },
    { method: 'POST', color: 'primary', path: '/orders' },
    { method: 'GET', color: 'success', path: '/orders/{id}' },
  ] as const;

  return (
    <Box
      aria-hidden
      data-testid="ready-to-connect"
      ref={frameRef}
      sx={{ height: HEIGHT * scale, maxWidth: '100%', width: WIDTH }}
    >
      <Box
        sx={{
          height: HEIGHT,
          position: 'relative',
          textAlign: 'left',
          transform: `scale(${scale})`,
          transformOrigin: 'top left',
          width: WIDTH,
        }}
      >
        <Box
          component="svg"
          fill="none"
          height={HEIGHT}
          sx={{ inset: 0, position: 'absolute' }}
          viewBox={`0 0 ${WIDTH} ${HEIGHT}`}
          width={WIDTH}
        >
          <Box component="g" sx={{ color: 'primary.main' }}>
            <g opacity={0.7} stroke="currentColor" strokeWidth={1.5}>
              <path d="M156 34 C190 34 186 84 216 84" />
              <path d="M156 84 L216 84" />
              <path d="M156 134 C190 134 186 84 216 84" />
            </g>
            <g fill="currentColor">
              <circle cx={156} cy={34} r={3} />
              <circle cx={156} cy={84} r={3} />
              <circle cx={156} cy={134} r={3} />
            </g>
          </Box>
          <Box component="g" sx={{ color: 'text.secondary' }}>
            <path
              d="M264 84 L304 84"
              stroke="currentColor"
              strokeDasharray="4 4"
              strokeWidth={1.5}
            />
            <path
              d="M298 79 L304 84 L298 89"
              stroke="currentColor"
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={1.5}
            />
          </Box>
        </Box>

        <Box
          sx={{
            display: 'flex',
            flexDirection: 'column',
            gap: '18px',
            left: 0,
            position: 'absolute',
            top: 0,
            width: 156,
          }}
        >
          {rows.map((row) => (
            <Box key={`${row.method}${row.path}`} sx={resourceSx}>
              <Box sx={verbSx(row.color)}>{row.method}</Box>
              <Box sx={{ color: 'text.primary' }}>{row.path}</Box>
            </Box>
          ))}
        </Box>

        <Box
          sx={(theme) => ({
            alignItems: 'center',
            bgcolor: alpha(theme.palette.primary.main, 0.35),
            border: 1,
            borderColor: alpha(theme.palette.primary.main, 0.6),
            borderRadius: 0.75,
            color: 'text.primary',
            display: 'flex',
            fontFamily: 'monospace',
            fontSize: 14,
            fontWeight: 600,
            height: 48,
            justifyContent: 'center',
            left: 216,
            position: 'absolute',
            top: 60,
            width: 48,
          })}
        >
          {HUB_LABEL}
        </Box>

        <Box
          sx={(theme) => ({
            bgcolor: alpha(theme.palette.text.primary, 0.14),
            border: 1,
            borderColor: 'divider',
            borderRadius: 0.75,
            boxSizing: 'border-box',
            display: 'flex',
            flexDirection: 'column',
            gap: 1.25,
            left: 308,
            p: 1.5,
            position: 'absolute',
            top: 30,
            width: 160,
          })}
        >
          <Box sx={{ alignItems: 'center', display: 'flex', gap: 1 }}>
            <Box
              sx={(theme) => ({
                alignItems: 'center',
                bgcolor: alpha(theme.palette.text.primary, 0.1),
                borderRadius: 0.5,
                display: 'flex',
                height: 22,
                justifyContent: 'center',
                width: 22,
              })}
            >
              <Server size={12} />
            </Box>
            <Box sx={{ flex: 1, fontSize: 12, fontWeight: 500 }}>
              {intl.formatMessage(messages.backend)}
            </Box>
          </Box>
          <Box
            sx={(theme) => ({
              alignItems: 'center',
              bgcolor: alpha(theme.palette.common.black, 0.2),
              border: 1,
              borderColor: alpha(theme.palette.primary.main, 0.55),
              borderRadius: 0.5,
              borderStyle: 'dashed',
              boxSizing: 'border-box',
              color: 'text.secondary',
              display: 'flex',
              fontFamily: 'monospace',
              fontSize: 10.5,
              gap: 0.75,
              height: 28,
              px: 1,
            })}
          >
            {URL_SCHEME}
            <Box sx={{ bgcolor: 'primary.main', height: 12, width: 2 }} />
          </Box>
        </Box>
      </Box>
    </Box>
  );
}
