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
import { defineMessages, FormattedMessage, FormattedNumber } from 'react-intl';

import { illustrationCardSx } from './illustrationCardSx';
import { useScaleToWidth } from './useScaleToWidth';
import './ApiDesignerCanvasIllustration.css';

const messages = defineMessages({
  aiReadiness: {
    id: 'apiControlPlane.components.ApiDesignerCanvasIllustration.aiReadiness',
    defaultMessage: 'AI-readiness',
    description: 'Label beside the score in the animated API Designer preview.',
  },
  openApiValid: {
    id: 'apiControlPlane.components.ApiDesignerCanvasIllustration.openApiValid',
    defaultMessage: 'OpenAPI 3.x valid',
    description: 'A passed check in the animated API Designer preview.',
  },
  spectralPassed: {
    id: 'apiControlPlane.components.ApiDesignerCanvasIllustration.spectralPassed',
    defaultMessage: 'Spectral rules passed',
    description: 'A passed check in the animated API Designer preview. Spectral is a tool name.',
  },
});

/** The animation is drawn at this size and scaled to whatever box it is given. */
const ART_WIDTH = 420;
const ART_HEIGHT = 236;
const SCORE = 0.92;

/**
 * The API Designer editor in motion: code lines write themselves in, a scan
 * bar flags one line, then a governance panel ticks off its checks and counts
 * up an AI-readiness score. It sits beside the hand-off to the API Designer
 * extension to show what that opens into.
 *
 * The artwork is laid out at a fixed 420x236 and scaled to fit its container,
 * so its type and spacing keep their proportions at any width. Colours come
 * from the theme through `--apid-*` custom properties rather than literals, so
 * it follows the light/dark toggle.
 *
 * Purely decorative: the surrounding copy carries the meaning, so it is hidden
 * from assistive technology. Its motion stops for anyone who prefers less.
 */
export function ApiDesignerCanvasIllustration() {
  const { ref: frameRef, scale } = useScaleToWidth(ART_WIDTH);

  return (
    <Box
      aria-hidden
      ref={frameRef}
      className="apid-frame"
      sx={(theme) => {
        const palette = theme.vars?.palette ?? theme.palette;
        const mix = (colour: string, share: number, base = 'transparent') =>
          `color-mix(in srgb, ${colour} ${share}%, ${base})`;
        return {
          // Theme variables and the tinted light-mode card, shared with the
          // other empty-state animations.
          ...illustrationCardSx('apid', theme),
          '--apid-accent-glow': mix(palette.primary.main, 15),
          '--apid-accent-line': mix(palette.primary.main, 53),
          '--apid-accent-soft': mix(palette.primary.main, 20),
          '--apid-accent-strong': palette.primary.light,
          '--apid-amber': palette.warning.main,
          '--apid-chrome': mix(palette.text.primary, 4, palette.background.paper),
          '--apid-dim': mix(palette.text.primary, 18, palette.background.paper),
          '--apid-dot': mix(palette.text.primary, 10),
          '--apid-faint': palette.text.disabled,
          '--apid-on-accent': palette.primary.contrastText,
          '--apid-panel': mix(palette.text.primary, 6, palette.background.paper),
          '--apid-shadow': mix('black', 35),
          ...theme.applyStyles('dark', {
            backgroundColor: palette.background.paper,
            border: '1px solid',
            borderColor: palette.divider,
          }),
          aspectRatio: `${ART_WIDTH} / ${ART_HEIGHT}`,
          borderRadius: 0.5,
          maxWidth: ART_WIDTH,
          overflow: 'hidden',
          position: 'relative',
          width: '100%',
        };
      }}
    >
      <Box
        className="apid-anim"
        sx={{
          left: 0,
          position: 'absolute',
          top: 0,
          transform: `scale(${scale})`,
          transformOrigin: 'top left',
        }}
      >
        <div className="apid-top">
          <i />
          <i />
          <i />
        </div>
        <div className="apid-body">
          <div className="apid-side">
            <i />
            <i className="on" />
            <i />
          </div>
          <div className="apid-code">
            <div className="apid-line">
              <b className="k" style={{ width: 44 }} />
              <b className="m" style={{ width: 30 }} />
            </div>
            <div className="apid-line">
              <b className="k" style={{ width: 30 }} />
            </div>
            <div className="apid-line">
              <b style={{ width: 56 }} />
              <b className="m" style={{ width: 40 }} />
            </div>
            <div className="apid-line flag">
              <b style={{ width: 38 }} />
              <b className="m" style={{ width: 64 }} />
            </div>
            <div className="apid-line">
              <b className="k" style={{ width: 40 }} />
            </div>
            <div className="apid-line">
              <b style={{ width: 48 }} />
              <b className="m" style={{ width: 52 }} />
            </div>
            <div className="apid-line">
              <b style={{ width: 34 }} />
              <b className="m" style={{ width: 26 }} />
            </div>
            <div className="apid-scan" />
            <div className="apid-panel">
              <div className="apid-row">
                <i />
                <FormattedMessage {...messages.openApiValid} />
              </div>
              <div className="apid-row">
                <i />
                <FormattedMessage {...messages.spectralPassed} />
              </div>
              <div className="apid-score">
                <div className="apid-ring" />
                <span>
                  <FormattedMessage {...messages.aiReadiness} />
                  <strong>
                    <FormattedNumber style="percent" value={SCORE} />
                  </strong>
                </span>
              </div>
            </div>
          </div>
        </div>
      </Box>
    </Box>
  );
}
