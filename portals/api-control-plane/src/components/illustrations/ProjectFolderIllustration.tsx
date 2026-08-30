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
 * A pair of stacked project folders, the front one holding a filled-in sheet —
 * the "here is what this page holds once you create something" illustration for
 * the first-run `EmptyState` on the project listing, and the counterpart to
 * `MonitorIllustration` on the API listing.
 *
 * Drawn through Oxygen's `ColorSchemeSVG` so every fill resolves to a theme
 * colour rather than a hex literal. The folder body is `text-primary`, which
 * means it reads as a dark folder in light mode and inverts to a light one in
 * dark mode — its contrast against the page, and against the `background`-filled
 * sheet inside it, holds either way.
 *
 * Purely decorative: the `EmptyState` heading and body carry the meaning, so
 * it is hidden from assistive technology.
 */
export function ProjectFolderIllustration() {
  return (
    <ColorSchemeSVG aria-hidden height={140} viewBox="0 0 220 140" width={220}>
      {/* The next folder in the stack, showing only as an edge behind. */}
      <rect fill="muted" height={76} rx={6} width={146} x={40} y={28} />

      {/* Front folder: the tab, then the back panel everything sits against. */}
      <rect fill="text-primary" height={22} rx={4} width={52} x={24} y={24} />
      <rect fill="text-primary" height={76} rx={6} width={150} x={24} y={40} />

      {/* The sheet filed inside, standing proud of the pocket: a brand mark, a
          title, and two lines of body copy. Filled with the page background so
          it stays legible whichever way the folder body resolves. */}
      <rect fill="background" height={46} rx={4} width={100} x={60} y={32} />
      <rect fill="primary" height={10} rx={2} width={10} x={70} y={40} />
      <rect fill="text-primary" height={5} rx={2} width={40} x={86} y={43} />
      <rect fill="muted" height={5} rx={2} width={76} x={70} y={56} />
      <rect fill="muted" height={5} rx={2} width={52} x={70} y={66} />

      {/* Front pocket, carrying a row of project chips. */}
      <rect fill="primary" height={42} rx={6} width={150} x={24} y={74} />
      <rect fill="background" height={8} rx={4} width={34} x={38} y={90} />
      <rect fill="background" height={8} rx={4} width={34} x={80} y={90} />
      <rect fill="background" height={8} rx={4} width={28} x={122} y={90} />

      {/* The surface it rests on. */}
      <path d="M50 128h120" stroke="border" strokeLinecap="round" strokeWidth={2} />
    </ColorSchemeSVG>
  );
}
