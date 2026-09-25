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

import { alpha, Box, Stack, Typography, type Theme } from '@wso2/oxygen-ui';
import { ChevronDown } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, useIntl } from 'react-intl';

import { methodPalette } from '@/components/SwaggerOperationsView';
import { hairline } from '@/theme/receipes';

const messages = defineMessages({
  description: {
    id: 'api.create.apiResourcesPreview.empty.body',
    defaultMessage: 'Import a contract to explore its endpoints',
  },
  title: {
    id: 'api.create.apiResourcesPreview.empty.title',
    defaultMessage: 'Resources will show here',
  },
});

/** The solid badge fill, the row's tinted background, and its 1px rule. */
export type PlaceholderRowTone = {
  badge: string;
  bg: string;
  border: string;
};

/**
 * A row in the mock listing — the entry points a definition almost always
 * starts with, so the pane reads as a list of them before it holds any real
 * one. `tone` is resolved against the theme rather than baked in as a plain
 * object, so a caller using theme-dependent colours (an Oxygen palette token)
 * works the same as REST's fixed, theme-independent Swagger verb colours.
 *
 * `ghost` is the tail of the list fading out — it says "and more below"
 * without pretending to know what.
 */
export type PlaceholderRow = {
  ghost?: boolean;
  label: string;
  tone: (theme: Theme) => PlaceholderRowTone;
};

/** REST's own default rows: the HTTP verbs a contract almost always starts with. */
const REST_PLACEHOLDER_ROWS: PlaceholderRow[] = [
  { label: 'GET', tone: () => methodPalette('GET') },
  { label: 'POST', tone: () => methodPalette('POST') },
  { label: 'PUT', tone: () => methodPalette('PUT') },
  { ghost: true, label: 'DELETE', tone: () => methodPalette('DELETE') },
];

/** Bounded so the copy underneath stays on two lines at the pane's width. */
const CONTENT_MAX_WIDTH = 320;

/** Widths of the two bars standing in for a path and a summary. */
const BAR_SHORT_WIDTH = '22%';
const BAR_LONG_WIDTH = '56%';

/** Fits the longest REST verb, "OPTIONS" — `minWidth`, so a longer label (a GraphQL operation kind) still grows past it. */
const BADGE_WIDTH = 64;
const BADGE_HEIGHT = 30;

/**
 * One of the two bars standing in for a row's text. Paper-coloured rather than
 * tinted, so it reads as an empty slot cut out of the row instead of a second
 * piece of content.
 */
const barSx = (width: string) => (theme: Theme) => ({
  bgcolor: alpha(theme.palette.text.primary, 0.18),
  borderRadius: 999,
  flexShrink: 0,
  height: 6,
  width,
});

export type ResourcePreviewPlaceholderProps = {
  /** Overrides the default explanation under the title. */
  description?: string;
  /** Overrides the default REST GET/POST/PUT/DELETE mock rows. */
  rows?: PlaceholderRow[];
  /** Hook for tests; also the element's `data-testid`. */
  testId?: string;
  /** Overrides the default heading over the explanation. */
  title?: string;
};

/**
 * The empty state of the resources pane: a mock listing of operations with the
 * explanation laid over it.
 *
 * Deliberately a *shape* rather than an illustration or a bare sentence — the
 * pane's whole job is to hold a list of operations, so showing that list in
 * outline tells the reader what the step produces before they've imported
 * anything. Everything in it is decorative (`aria-hidden` on the mock rows);
 * only the title and description are announced.
 */
export const ResourcePreviewPlaceholder = ({
  description,
  rows = REST_PLACEHOLDER_ROWS,
  testId = 'resource-preview-placeholder',
  title,
}: ResourcePreviewPlaceholderProps) => {
  const intl = useIntl();

  return (
    <Box
      data-testid={testId}
      sx={(theme) => ({
        alignItems: 'center',
        bgcolor: alpha(theme.palette.text.primary, 0.025),
        backgroundImage: `radial-gradient(circle at 50% 15%, ${alpha(
          theme.palette.primary.main,
          0.06,
        )}, transparent 42%)`,
        border: hairline(theme),
        borderColor: 'divider',
        borderRadius: 2,
        display: 'flex',
        justifyContent: 'center',
        // `minHeight` rather than `height`: it fills a short pane, but a tall
        // enough one lets the content set the height instead of clipping it.
        minHeight: '100%',
        overflow: 'hidden',
        p: { sm: 3, xs: 2.25 },
        position: 'relative',
      })}
    >
      <Stack
        sx={{
          alignItems: 'center',
          maxWidth: { sm: CONTENT_MAX_WIDTH, xs: '100%' },
          // Above the glows, which are absolutely positioned siblings.
          position: 'relative',
          width: '100%',
          zIndex: 1,
        }}
      >
        <Stack aria-hidden spacing={1} sx={{ mb: { sm: 4, xs: 3 }, width: '100%' }}>
          {rows.map((row) => (
            <Stack
              direction="row"
              key={row.label}
              spacing={1.2}
              sx={(theme) => {
                const tone = row.tone(theme);

                return {
                  alignItems: 'center',
                  bgcolor: tone.bg,
                  border: hairline(theme),
                  borderColor: tone.border,
                  borderRadius: 0.75,
                  boxShadow: 'none',
                  minHeight: { sm: 40, xs: 38 },
                  px: 1.35,
                  py: 0.9,
                  // The tail of the list, trailing off rather than ending.
                  ...(row.ghost && { opacity: 0.35, width: '86%' }),
                };
              }}
            >
              <Box
                sx={(theme) => ({
                  alignItems: 'center',
                  bgcolor: row.tone(theme).badge,
                  borderRadius: 0.5,
                  color: 'common.white',
                  display: 'inline-flex',
                  flexShrink: 0,
                  fontSize: 12,
                  fontWeight: 700,
                  height: BADGE_HEIGHT,
                  justifyContent: 'center',
                  letterSpacing: 0.35,
                  minWidth: BADGE_WIDTH,
                  px: 1.25,
                  textTransform: 'uppercase',
                })}
              >
                {row.label}
              </Box>
              <Stack
                direction="row"
                spacing={1}
                sx={{ alignItems: 'center', flex: 1, minWidth: 0 }}
              >
                <Box sx={barSx(BAR_SHORT_WIDTH)} />
                <Box sx={barSx(BAR_LONG_WIDTH)} />
              </Stack>
              <Box
                sx={(theme) => ({
                  // ChevronDown paints in `currentColor`, so tinting the
                  // wrapper is what colours the glyph. Text-coloured, not
                  // method-coloured: the real rows draw theirs the same way.
                  color: alpha(theme.palette.text.primary, 0.5),
                  display: 'flex',
                  flexShrink: 0,
                })}
              >
                <ChevronDown size={17} />
              </Box>
            </Stack>
          ))}
        </Stack>

        <Stack spacing={0.5} sx={{ textAlign: 'center' }}>
          <Typography sx={{ fontWeight: 700 }} variant="body1">
            {title ?? intl.formatMessage(messages.title)}
          </Typography>
          <Typography color="text.secondary" variant="body2">
            {description ?? intl.formatMessage(messages.description)}
          </Typography>
        </Stack>
      </Stack>
    </Box>
  );
};
