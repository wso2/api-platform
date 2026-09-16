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

import { alpha, toggleButtonGroupClasses, type Theme } from '@wso2/oxygen-ui';

/**
 * The `border` shorthand for a one-pixel rule, from `theme.border` rather than
 * a `'1px solid'` literal. Pair it with a `borderColor` token — the colour is
 * the part that actually varies between light, dark and high-contrast themes.
 */
export const hairline = (theme: Theme) => `${theme.border.width} ${theme.border.style}`;

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
      0.6,
    )}, ${alpha(theme.palette.background.paper, 0.25)})`,
    border: hairline(theme),
    borderColor: alpha(theme.palette.divider, 0.6),
    borderRadius: 1,
  }) as const;

/**
 * Blur radius of a decorative colour wash. One value so every glow in a panel
 * reads as the same light source rather than three unrelated smudges.
 */
const AMBIENT_GLOW_BLUR = '34px';

/**
 * A soft, blurred colour wash behind a decorative panel; the empty-state
 * preview pane's ambient lighting.
 *
 * Only the parts that never vary live here: the blur, the circular shape, and
 * taking the element out of both flow and hit-testing. Each call site supplies
 * its own position, size and `bgcolor` (an `alpha()` of a palette token), since
 * those are what place one glow versus another.
 */
export const ambientGlowSx = {
  borderRadius: '50%',
  filter: `blur(${AMBIENT_GLOW_BLUR})`,
  pointerEvents: 'none',
  position: 'absolute',
} as const;

/**
 * Hover treatment for a card that behaves as a button (the whole surface
 * navigates). Deliberately NOT a global `MuiCard` override: non-interactive
 * cards — the tab shell, the save bar, the Explore More panel — must not lift
 * under the cursor.
 */
export const interactiveCardSx = {
  cursor: 'pointer',
  transition: 'transform .18s ease, border-color .18s ease, box-shadow .18s ease',
  '&:hover': {
    borderColor: 'primary.main',
    boxShadow: 4,
    transform: 'translateY(-3px)',
  },
} as const;

/**
 * Focus indicator for a card or row that is itself the control (see
 * `openableProps` in `components/openable.ts`). Inset by its own width so the
 * ring stays inside a card that clips its overflow, and keyed to
 * `:focus-visible` so a pointer click doesn't leave a ring behind.
 */
export const focusRingSx = (theme: Theme) => ({
  '&:focus-visible': {
    outline: `2px solid ${theme.palette.primary.main}`,
    outlineOffset: '-2px',
  },
});

/**
 * State layer for a card the user picks from a set of options: the API
 * creation wizard's API-type and Gateway Creation's Gateway-type Card.
 *
 * `Form.CardButton` already owns the hover treatment; this adds the part that
 * depends on *state*: a primary-tinted ring on the current choice, and a flat,
 * dimmed surface for an option that is visible but cannot be picked (not yet
 * released, or not offered by the selected proxy type). Disabling the click is
 * the `disabled` prop's job, this only makes the state legible.
 */
export const selectableCardSx = (theme: Theme, state: { disabled?: boolean; selected?: boolean }) =>
  ({
    borderColor: state.selected ? 'primary.main' : 'divider',
    ...(state.selected && {
      backgroundColor: alpha(theme.palette.primary.main, 0.06),
      boxShadow: theme.shadows[1],
    }),
    ...(state.disabled && {
      backgroundColor: 'transparent',
      borderColor: 'divider',
      boxShadow: 'none',
      opacity: 0.55,
    }),
  }) as const;

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

/**
 * Fully-rounded ends. No radius token goes this far — `theme.shape` tops out at
 * a card's corner — so the pill shape is defined once, here.
 */
const PILL_RADIUS = '999px';

/**
 * Subtle light-mode elevation for the selected segment. The flat theme does
 * not provide a suitable elevation token, so the value is defined here once;
 * dark mode disables it.
 */
const SEGMENT_SHADOW = '0 1px 2px rgba(0, 0, 0, 0.12)';

/**
 * Styles a pill-shaped segmented switch for mutually exclusive views.
 * Uses `theme.vars` tokens so track, active-segment, and text colours follow
 * the active colour scheme. Grouped-button borders are removed to preserve
 * the pill shape.
 */
export const segmentedSwitchSx = (theme: Theme) => {
  const palette = theme.vars?.palette ?? theme.palette;

  return {
    backgroundColor: palette.action.hover,
    borderRadius: PILL_RADIUS,
    gap: theme.spacing(0.5),
    padding: theme.spacing(0.5),
    [`& .${toggleButtonGroupClasses.grouped}`]: {
      border: 0,
      borderRadius: PILL_RADIUS,
      color: palette.text.secondary,
      fontWeight: theme.typography.fontWeightMedium,
      marginInline: 0,
      paddingInline: theme.spacing(2),
      textTransform: 'none',
      // Stronger than the track, or hovering an inactive segment lands on the
      // colour the track already is and reads as dead.
      '&:hover': {
        backgroundColor: palette.action.selected,
      },
      '&.Mui-selected': {
        backgroundColor: palette.background.paper,
        borderRadius: PILL_RADIUS,
        boxShadow: SEGMENT_SHADOW,
        color: palette.primary.main,
        fontWeight: theme.typography.fontWeightBold,
        ...theme.applyStyles('dark', { boxShadow: 'none' }),
        // Without this the active pill loses its fill on hover and the control
        // flickers as the cursor crosses it.
        '&:hover': {
          backgroundColor: palette.background.paper,
        },
      },
    },
  } as const;
};
