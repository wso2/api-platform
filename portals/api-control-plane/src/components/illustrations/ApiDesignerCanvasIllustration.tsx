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

/** Dot-grid lattice behind the flow — sparse enough to read as a canvas. */
const GRID_COLUMNS = 13;
const GRID_ROWS = 7;

/**
 * A design canvas seen from a distance: window chrome, an activity rail, a dot
 * grid, and a flow of operation nodes. It sits behind the hand-off button in
 * `DesignWithAiPanel` to show what the API Designer extension opens into.
 *
 * Drawn through Oxygen's `ColorSchemeSVG` so every fill and stroke resolves to
 * a theme colour rather than a hex literal — the console has a light/dark
 * toggle, and a fixed-colour illustration would go muddy in dark mode.
 *
 * Purely decorative: the panel's heading and body carry the meaning, so it is
 * hidden from assistive technology.
 */
export function ApiDesignerCanvasIllustration() {
  return (
    <ColorSchemeSVG
      aria-hidden
      height="auto"
      sx={{ display: 'block', width: '100%' }}
      viewBox="0 0 320 180"
    >
      {/* Editor window: chrome bar over the canvas body. The second rect
          squares off the chrome's lower corners, which the rounded one keeps. */}
      <rect fill="surface" height={180} rx={8} width={320} />
      <rect fill="muted" height={20} rx={8} width={320} />
      <rect fill="muted" height={12} width={320} y={8} />
      <g fill="border">
        <circle cx={14} cy={10} r={3} />
        <circle cx={26} cy={10} r={3} />
        <circle cx={38} cy={10} r={3} />
      </g>

      {/* Activity rail down the left edge, one item active. */}
      <rect fill="muted" height={160} width={24} x={0} y={20} />
      <rect fill="border" height={10} rx={2} width={10} x={7} y={32} />
      <rect fill="primary" height={10} rx={2} width={10} x={7} y={48} />
      <rect fill="border" height={10} rx={2} width={10} x={7} y={64} />

      <g fill="border">
        {Array.from({ length: GRID_ROWS }, (_, row) =>
          Array.from({ length: GRID_COLUMNS }, (_, column) => (
            <circle cx={40 + column * 22} cy={36 + row * 20} key={`${row}-${column}`} r={1} />
          )),
        )}
      </g>

      {/* The flow itself: a start pill, an operation card with a branch off it,
          a decision diamond, and a placeholder waiting to be filled in. */}
      <g fill="none" stroke="border" strokeWidth={1.5}>
        <path d="M170 52v16M170 104v14M170 140v12" />
        <rect height={22} rx={11} width={54} x={143} y={30} />
        <rect fill="background" height={36} rx={6} width={110} x={115} y={68} />
        <path d="M225 86h7" />
        <circle cx={244} cy={86} r={12} />
        <rect
          fill="background"
          height={22}
          rx={4}
          transform="rotate(45 170 129)"
          width={22}
          x={159}
          y={118}
        />
        <rect height={22} rx={6} strokeDasharray="4 3" width={110} x={115} y={152} />
      </g>

      {/* Contents of the operation card, and the two accents that give the
          canvas a focal point. */}
      <g fill="border">
        <rect height={8} rx={2} width={8} x={125} y={82} />
        <rect height={6} rx={3} width={48} x={139} y={83} />
        <rect height={8} rx={2} width={8} x={205} y={82} />
      </g>
      <circle cx={244} cy={86} fill="primary" r={5} />
      <path d="M166 163h8M170 159v8" stroke="primary" strokeLinecap="round" strokeWidth={1.5} />
    </ColorSchemeSVG>
  );
}
