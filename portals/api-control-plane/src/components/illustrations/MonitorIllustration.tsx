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

import { ColorSchemeSVG } from '@wso2/oxygen-ui';

/**
 * A monitor showing a filled-in console — the "here is what this page looks
 * like once you create something" illustration for a first-run `EmptyState`.
 *
 * Drawn through Oxygen's `ColorSchemeSVG` so every fill resolves to a theme
 * colour rather than a hex literal. The screen body is `text-primary`, which
 * means it reads as a dark screen in light mode and inverts to a light one in
 * dark mode — the contrast against the page, and against the accent content
 * inside it, holds either way.
 *
 * Purely decorative: the `EmptyState` heading and body carry the meaning, so
 * it is hidden from assistive technology.
 */
export function MonitorIllustration() {
  return (
    <ColorSchemeSVG aria-hidden height={140} viewBox="0 0 220 140" width={220}>
      {/* Screen body. Everything below sits inside it. */}
      <rect fill="text-primary" height={112} rx={4} width={192} x={14} y={6} />

      {/* Left pane: a brand mark over a stack of list rows. */}
      <rect fill="primary" height={10} rx={2} width={10} x={26} y={20} />
      <rect fill="muted" height={5} rx={2} width={44} x={42} y={23} />
      <rect fill="primary" height={6} rx={3} width={68} x={26} y={42} />
      <rect fill="primary" height={6} rx={3} width={52} x={26} y={54} />
      <rect fill="primary" height={6} rx={3} width={62} x={26} y={66} />
      <rect fill="muted" height={6} rx={3} width={44} x={26} y={78} />

      {/* Right pane, top: a card grid. */}
      <rect fill="primary" height={10} rx={2} width={22} x={120} y={20} />
      <rect fill="primary" height={10} rx={2} width={22} x={146} y={20} />
      <rect fill="primary" height={10} rx={2} width={22} x={172} y={20} />
      <rect fill="primary" height={10} rx={2} width={22} x={120} y={34} />
      <rect fill="primary" height={10} rx={2} width={22} x={146} y={34} />
      <rect fill="primary" height={10} rx={2} width={22} x={172} y={34} />
      <rect fill="primary" height={10} rx={2} width={22} x={120} y={48} />
      <rect fill="primary" height={10} rx={2} width={22} x={146} y={48} />
      <rect fill="primary" height={10} rx={2} width={22} x={172} y={48} />

      {/* Right pane, bottom: a detail panel, filled with the page background so
          it stays legible whichever way the screen body resolves. */}
      <rect fill="background" height={38} rx={3} width={76} x={118} y={64} />
      <rect fill="text-primary" height={5} rx={2} width={40} x={126} y={72} />
      <rect fill="primary" height={5} rx={2} width={28} x={126} y={82} />
      <rect fill="muted" height={4} rx={2} width={52} x={126} y={92} />

      {/* Stand and surface use `muted` so the line stays visible in both themes. */}
      <rect fill="text-disabled" height={12} rx={1} width={24} x={98} y={118} />
      <path d="M62 132h96" stroke="muted" strokeLinecap="round" strokeWidth={2} />
    </ColorSchemeSVG>
  );
}
