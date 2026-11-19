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

import { Box, Stack, Typography } from '@wso2/oxygen-ui';
import { ArrowLeftRight, Globe, Plug, RadioTower, Share2 } from '@wso2/oxygen-ui-icons-react';
import { useEffect, useState, type ReactElement } from 'react';
import { defineMessages, FormattedMessage } from 'react-intl';

import { usePrefersReducedMotion } from '@/hooks/usePrefersReducedMotion';

import { API_TYPES } from '../../apis/create/uiConfig';

const ROW_HEIGHT_REM = 2;
const ROW_HEIGHT = `${ROW_HEIGHT_REM}rem`;
const HOLD_MS = 2200;
const SLIDE_MS = 500;

const messages = defineMessages({
  supports: {
    id: 'apiControlPlane.pages.appShell.appShellPages.projects.ApiTypeTicker.supports',
    defaultMessage: 'Supports',
    description: 'Lead-in to a rotating list of the API types the product supports.',
  },
  rest: {
    id: 'apiControlPlane.pages.appShell.appShellPages.projects.ApiTypeTicker.rest',
    defaultMessage: 'REST APIs',
  },
  graphql: {
    id: 'apiControlPlane.pages.appShell.appShellPages.projects.ApiTypeTicker.graphql',
    defaultMessage: 'GraphQL APIs',
  },
  websocket: {
    id: 'apiControlPlane.pages.appShell.appShellPages.projects.ApiTypeTicker.websocket',
    defaultMessage: 'WebSocket APIs',
  },
  websub: {
    id: 'apiControlPlane.pages.appShell.appShellPages.projects.ApiTypeTicker.websub',
    defaultMessage: 'WebSub APIs',
  },
  grpc: {
    id: 'apiControlPlane.pages.appShell.appShellPages.projects.ApiTypeTicker.grpc',
    defaultMessage: 'gRPC APIs',
  },
});

const TYPE_ROWS: Record<string, { icon: ReactElement; message: keyof typeof messages }> = {
  rest: { icon: <Globe size={18} />, message: 'rest' },
  graphql: { icon: <Share2 size={18} />, message: 'graphql' },
  websocket: { icon: <ArrowLeftRight size={18} />, message: 'websocket' },
  websub: { icon: <RadioTower size={18} />, message: 'websub' },
  grpc: { icon: <Plug size={18} />, message: 'grpc' },
};

/**
 * "Supports" followed by the API types of the create wizard's catalog,
 * scrolling up one at a time, so a type added there shows up here without a
 * second edit. The first row repeats at
 * the end so the last scroll lands on it, then the strip snaps back unseen.
 */
export function ApiTypeTicker() {
  const reducedMotion = usePrefersReducedMotion();
  const rows = API_TYPES.filter((type) => TYPE_ROWS[type.key]).map((type) => TYPE_ROWS[type.key]);
  const count = rows.length;
  const [index, setIndex] = useState(0);
  const [snapping, setSnapping] = useState(false);
  const animate = count > 1 && !reducedMotion;

  useEffect(() => {
    if (!animate) return undefined;
    const advance = setTimeout(() => setIndex((current) => current + 1), HOLD_MS);
    return () => clearTimeout(advance);
  }, [animate, index]);

  useEffect(() => {
    if (index !== count) return undefined;
    const snap = setTimeout(() => {
      setSnapping(true);
      setIndex(0);
    }, SLIDE_MS);
    return () => clearTimeout(snap);
  }, [index, count]);

  useEffect(() => {
    if (!snapping) return undefined;
    // Two frames: the snapped position has to be painted before the transition
    // comes back, or the strip would scroll backwards through every row.
    let second = 0;
    const first = requestAnimationFrame(() => {
      second = requestAnimationFrame(() => setSnapping(false));
    });
    return () => {
      cancelAnimationFrame(first);
      cancelAnimationFrame(second);
    };
  }, [snapping]);

  if (count === 0) return null;
  const strip = count > 1 ? [...rows, rows[0]] : rows;

  return (
    <Stack alignItems="center" direction="row" spacing={1.5}>
      <Typography color="text.secondary" variant="body1">
        <FormattedMessage {...messages.supports} />
      </Typography>
      <Box sx={{ height: ROW_HEIGHT, overflow: 'hidden' }}>
        <Box
          // Inline: it changes with every step, and a class per step would pile up.
          style={{
            transform: `translateY(-${index * ROW_HEIGHT_REM}rem)`,
            transition: snapping ? 'none' : `transform ${SLIDE_MS}ms ease-in-out`,
          }}
        >
          {strip.map((row, position) => (
            <Box
              // The repeated first row only keeps the loop seamless; position keeps keys unique.
              aria-hidden={count > 1 && position === count ? true : undefined}
              key={`${row.message}-${position}`}
              sx={{
                alignItems: 'center',
                color: 'primary.main',
                display: 'flex',
                gap: 0.75,
                height: ROW_HEIGHT,
              }}
            >
              {row.icon}
              <Typography sx={{ fontWeight: 500 }} variant="body1">
                <FormattedMessage {...messages[row.message]} />
              </Typography>
            </Box>
          ))}
        </Box>
      </Box>
    </Stack>
  );
}
