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

// src/theme/recipes.ts
//
// Shared style recipes — the middle tier between the Oxygen theme and one-off
// layout `sx`.
//
//   1. Theme      — global design decisions, owned by @wso2/oxygen-ui (and any
//                   app-level override registered in ./themes).
//   2. Recipes    — repeated multi-property treatments that are too
//                   instance-specific to be a global component override. THIS
//                   FILE. One definition, imported by every call site.
//   3. Local `sx` — layout only (flex, gap, grid columns, min/max sizing).
//
// Everything here resolves through theme tokens. No colour, radius, blur or
// border literals belong in this file or in any call site.

import { alpha, type Theme } from '@wso2/oxygen-ui';

/**
 * The `border` shorthand for a one-pixel rule, from `theme.border` rather than
 * a `'1px solid'` literal. Pair it with a `borderColor` token — the colour is
 * the part that actually varies between light, dark and high-contrast themes.
 */
export const hairline = (theme: Theme) =>
  `${theme.border.width} ${theme.border.style}`;

/**
 * Square metadata chip used across the API and gateway card family. A function
 * of the theme so the border comes from `theme.border` rather than a literal —
 * compose it as `sx={(theme) => ({ ...chipSx(theme), ...overrides })}`.
 */
export const chipSx = (theme: Theme) =>
  ({
    alignItems: 'center',
    bgcolor: 'action.hover',
    border: hairline(theme),
    borderColor: 'divider',
    borderRadius: 1,
    color: 'text.secondary',
    display: 'inline-flex',
    fontSize: 12,
    fontWeight: 500,
    gap: 0.75,
    px: 1.25,
    py: 0.5,
  }) as const;

/**
 * The same chip tinted with the brand colour — the "kind" chip on API cards.
 * Pass straight through as `sx={tintedChipSx}`.
 */
export const tintedChipSx = (theme: Theme) => ({
  ...chipSx(theme),
  bgcolor: alpha(theme.palette.primary.main, 0.14),
  borderColor: alpha(theme.palette.primary.main, 0.3),
  color: 'primary.main',
  fontWeight: 600,
});

/** Blur radius behind a glass surface. One value, so every pane matches. */
const GLASS_BLUR = '14px';

/**
 * Translucent "glass" surface: what sits behind the element shows through,
 * blurred, instead of the flat `background.paper` fill.
 *
 * Both gradient stops derive from `background.paper`, so the sheen that makes
 * it read as glass stays correct in light, dark and high-contrast themes
 * without branching on the palette mode. Compose it as
 * `sx={(theme) => ({ ...glassSurfaceSx(theme), ...layout })}`.
 */
export const glassSurfaceSx = (theme: Theme) =>
  ({
    backdropFilter: `blur(${GLASS_BLUR})`,
    WebkitBackdropFilter: `blur(${GLASS_BLUR})`,
    backgroundColor: 'transparent',
    backgroundImage: `linear-gradient(135deg, ${alpha(
      theme.palette.background.paper,
      0.6
    )}, ${alpha(theme.palette.background.paper, 0.25)})`,
    border: hairline(theme),
    borderColor: alpha(theme.palette.divider, 0.6),
    borderRadius: 1,
  }) as const;

/**
 * Hover treatment for a card that behaves as a button (the whole surface
 * navigates). Deliberately NOT a global `MuiCard` override: non-interactive
 * cards — the tab shell, the save bar, the Explore More panel — must not lift
 * under the cursor.
 */
export const interactiveCardSx = {
  cursor: 'pointer',
  transition:
    'transform .18s ease, border-color .18s ease, box-shadow .18s ease',
  '&:hover': {
    borderColor: 'primary.main',
    boxShadow: 4,
    transform: 'translateY(-3px)',
  },
} as const;

/**
 * Upward elevation for a bar that floats over scrolling content (the develop
 * tabs' sticky save bar). `theme.shadows` is entirely downward-casting, and the
 * duplicated @mui/material install blocks adding a typed custom theme token, so
 * this lives here as the single definition rather than inline at the call site.
 */
export const overlayBarShadow = '0 -2px 10px rgba(0, 0, 0, 0.16)';

/**
 * A bar pinned to the bottom of a page's scroll area — the develop tabs' save
 * bar, a list page's pagination row.
 *
 * Blurred, bordered and shadowed upward so the content scrolling underneath
 * stays readable behind it instead of colliding with it. It deliberately does
 * **not** set a `bottom` offset: the app footer shares this scroll area, so each
 * call site pairs this with `bottom: useFooterHeight()` rather than 0, or the bar
 * ends up behind the footer.
 */
export const stickyBottomBarSx = (theme: Theme) =>
  ({
    backdropFilter: 'blur(10px)',
    WebkitBackdropFilter: 'blur(10px)',
    borderColor: 'divider',
    borderTop: hairline(theme),
    boxShadow: overlayBarShadow,
    position: 'sticky',
    zIndex: theme.zIndex.appBar,
  }) as const;
