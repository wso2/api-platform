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
import { decomposeColor, GlobalStyles, useTheme, type Theme } from '@wso2/oxygen-ui';

/**
 * Bridges the console's Oxygen theme into GraphiQL.
 *
 * GraphiQL themes itself through CSS custom properties holding bare HSL
 * channels (`--color-primary: 320, 95%, 43%`, used as `hsl(var(--x))` and
 * `hsla(var(--x), alpha)`), declared on its container and on the dialogs and
 * tooltips it portals to `<body>`. This overrides them from the console's own
 * palette — once per colour scheme, keyed on the `data-color-scheme` attribute
 * Oxygen stamps on `<html>` — so the Test console reads as part of the app
 * rather than as GraphiQL's own pink-and-navy defaults.
 *
 * Token roles mirror the console's schema explorer: field and type names use
 * `info` (as the explorer colours types), argument names use plain text (as
 * the explorer sets them), and actions use the brand `primary`.
 */

/** Everything GraphiQL declares its colour variables on, portalled elements included. */
const GRAPHIQL_TARGETS = [
  '.graphiql-container',
  '.graphiql-dialog',
  '.graphiql-dialog-overlay',
  '.graphiql-tooltip',
  '[data-radix-popper-content-wrapper]',
];

/**
 * A colour as GraphiQL's bare `h, s%, l%` channels. Alpha is dropped: the
 * variables are composed as `hsl(…)`/`hsla(…, a)`, so they can only carry an
 * opaque colour (a translucent paper surface becomes its opaque base).
 */
export const toHslChannels = (color: string): string => {
  const { type, values } = decomposeColor(color);
  if (type.startsWith('hsl')) {
    return `${Math.round(values[0])}, ${Math.round(values[1])}%, ${Math.round(values[2])}%`;
  }
  const [r, g, b] = values.slice(0, 3).map((v) => v / 255);
  const max = Math.max(r, g, b);
  const min = Math.min(r, g, b);
  const lightness = (max + min) / 2;
  let hue = 0;
  let saturation = 0;
  if (max !== min) {
    const delta = max - min;
    saturation = lightness > 0.5 ? delta / (2 - max - min) : delta / (max + min);
    if (max === r) hue = (g - b) / delta + (g < b ? 6 : 0);
    else if (max === g) hue = (b - r) / delta + 2;
    else hue = (r - g) / delta + 4;
    hue *= 60;
  }
  return `${Math.round(hue)}, ${Math.round(saturation * 100)}%, ${Math.round(lightness * 100)}%`;
};

type Palette = Theme['palette'];

/** GraphiQL's colour variables, filled from one scheme's palette. */
export const graphiqlVariables = (palette: Palette, fontFamily?: string): Record<string, string> => ({
  '--color-primary': toHslChannels(palette.primary.main),
  '--color-secondary': toHslChannels(palette.text.primary),
  '--color-tertiary': toHslChannels(palette.info.main),
  '--color-info': toHslChannels(palette.info.main),
  '--color-success': toHslChannels(palette.success.main),
  '--color-warning': toHslChannels(palette.warning.main),
  '--color-error': toHslChannels(palette.error.main),
  '--color-neutral': toHslChannels(palette.text.primary),
  '--color-base': toHslChannels(palette.background.paper),
  ...(fontFamily ? { '--font-family': fontFamily } : {}),
});

/**
 * `prefix` + every target, each with its class/attribute doubled. GraphiQL's
 * own scheme rules are `body.graphiql-dark .graphiql-container` (0,2,1); the
 * doubling lifts ours to (0,3,1) so they win regardless of stylesheet order.
 */
const targetsUnder = (prefix: string) =>
  GRAPHIQL_TARGETS.map((target) => `${prefix}${target}${target}`).join(', ');

/**
 * GraphiQL colours type links with its `warning` token, the same token it
 * uses for deprecation notices, so recolouring `warning` itself would also
 * recolour real warnings. Point just the type links at `info` instead,
 * matching how the console's schema explorer colours types.
 */
const TYPE_LINK_STYLES = {
  '.graphiql-container a.graphiql-doc-explorer-type-name': { color: 'hsl(var(--color-info))' },
  '.graphiql-container a.graphiql-doc-explorer-type-name:focus': {
    outline: 'hsl(var(--color-info)) auto 1px',
  },
};

type SchemePalettes = { dark?: { palette?: Palette }; light?: { palette?: Palette } };

export function GraphiqlThemeStyles() {
  const theme = useTheme();
  const schemes = (theme as Theme & { colorSchemes?: SchemePalettes }).colorSchemes;
  const fontFamily = theme.typography.fontFamily;
  const light = schemes?.light?.palette;
  const dark = schemes?.dark?.palette;

  // A theme without colour schemes has one palette for both.
  const styles =
    light || dark
      ? {
          ...(light && {
            [targetsUnder("html[data-color-scheme='light'] ")]: graphiqlVariables(light, fontFamily),
          }),
          ...(dark && {
            [targetsUnder("html[data-color-scheme='dark'] ")]: graphiqlVariables(dark, fontFamily),
          }),
        }
      : { [targetsUnder('html ')]: graphiqlVariables(theme.palette, fontFamily) };

  return <GlobalStyles styles={{ ...styles, ...TYPE_LINK_STYLES }} />;
}
