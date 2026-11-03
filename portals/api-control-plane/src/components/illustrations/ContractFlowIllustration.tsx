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

import { Box } from '@wso2/oxygen-ui';
import type { CSSProperties } from 'react';
import { defineMessages, FormattedMessage } from 'react-intl';

import { methodPalette } from '@/components/SwaggerOperationsView/methodPalette';

import './ContractFlowIllustration.css';
import { useScaleToWidth } from './useScaleToWidth';

const messages = defineMessages({
  resourcesFound: {
    id: 'apiControlPlane.components.ContractFlowIllustration.resourcesFound',
    defaultMessage: '{count, plural, one {# resource found} other {# resources found}}',
    description: 'Status chip in the animated contract-import preview.',
  },
});

/** The animation is drawn at this size (chip included) and scaled to its box. */
const ART_WIDTH = 420;
const ART_HEIGHT = 234;
const CONTRACT_FORMAT = 'OPENAPI';

/** Sample operations the preview "finds" in the contract. Data, not copy. */
const RESOURCES = [
  { method: 'GET', path: '/users' },
  { method: 'POST', path: '/orders' },
  { method: 'PUT', path: '/products/{id}' },
] as const;

/**
 * A contract file dropping in, being scanned, and fanning out into the API
 * resources it describes. It fills the "Ready to connect" preview beside the
 * contract import form, to show what the imported contract turns into.
 *
 * Drawn at a fixed 420x234 and scaled to its container. Colours come from the
 * theme through `--apict-*` custom properties (darker accent shades in light
 * mode, lighter in dark) rather than literals, so it follows the light/dark
 * toggle.
 *
 * Purely decorative: the heading and text beside it carry the meaning, so it
 * is hidden from assistive technology. Its motion stops for anyone who prefers
 * less.
 */
export function ContractFlowIllustration() {
  const { ref, scale } = useScaleToWidth(ART_WIDTH);

  return (
    <Box
      aria-hidden
      ref={ref}
      sx={(theme) => {
        const palette = theme.vars?.palette ?? theme.palette;
        const mix = (colour: string, share: number, base = 'transparent') =>
          `color-mix(in srgb, ${colour} ${share}%, ${base})`;
        return {
          '--apict-accent': palette.primary.main,
          '--apict-accent-glow': mix(palette.primary.main, 13),
          '--apict-accent-strong': palette.primary.light,
          '--apict-accent-text': palette.primary.dark,
          '--apict-bg': palette.background.paper,
          '--apict-chip-bg': mix(palette.primary.main, 12, palette.background.paper),
          '--apict-chip-border': mix(palette.primary.main, 30, palette.background.paper),
          '--apict-dim': mix(palette.text.primary, 16, palette.background.paper),
          '--apict-faint': palette.text.disabled,
          '--apict-muted': palette.text.secondary,
          '--apict-on-accent': palette.primary.contrastText,
          '--apict-on-method': palette.common.white,
          '--apict-scan-fade': mix(palette.primary.light, 25),
          '--apict-shadow': mix('black', 12),
          '--apict-surface': mix(palette.text.primary, 5, palette.background.paper),
          '--apict-text': palette.text.primary,
          ...theme.applyStyles('dark', {
            '--apict-accent-text': palette.primary.light,
            '--apict-shadow': mix('black', 30),
          }),
          aspectRatio: `${ART_WIDTH} / ${ART_HEIGHT}`,
          maxWidth: ART_WIDTH,
          position: 'relative',
          width: '100%',
        };
      }}
    >
      <Box
        className="apict-anim"
        sx={{
          left: 0,
          position: 'absolute',
          top: 0,
          transform: `scale(${scale})`,
          transformOrigin: 'top left',
        }}
      >
        <div className="apict-file">
          <div className="apict-scan" />
          <span className="apict-file-tag">
            <i />
            {CONTRACT_FORMAT}
          </span>
          <b className="o" style={{ width: '60%' }} />
          <b className="l" style={{ width: '85%' }} />
          <b style={{ width: '70%' }} />
          <b className="l" style={{ width: '78%' }} />
          <b style={{ width: '50%' }} />
        </div>
        <div className="apict-wire">
          <span />
          <span />
          <span />
        </div>
        <div className="apict-branch" />
        {RESOURCES.map(({ method, path }, index) => (
          <div
            className={`apict-res apict-res-${index + 1}`}
            key={path}
            // The verb colours every other resource list in the console uses.
            style={{ '--apict-method': methodPalette(method).badge } as CSSProperties}
          >
            <em>{method}</em>
            {path}
          </div>
        ))}
        <div className="apict-chip">
          <i />
          <FormattedMessage {...messages.resourcesFound} values={{ count: RESOURCES.length }} />
        </div>
      </Box>
    </Box>
  );
}
