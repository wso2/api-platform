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

import { alpha, Box, Chip, Stack, Typography, type Theme } from '@wso2/oxygen-ui';
import { ChevronDown } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, useIntl } from 'react-intl';

import { ambientGlowSx, hairline } from '@/theme/receipes';
import { methodColor, type ChipColor } from '../utils/developEdit';

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

/**
 * A row in the mock listing. The methods a definition almost always starts
 * with, so the pane reads as a list of operations before it holds any.
 *
 * `ghost` is the tail of the list fading out — it says "and more below" without
 * pretending to know what.
 */
type PlaceholderRow = {
  ghost?: boolean;
  method: string;
};

const PLACEHOLDER_ROWS: PlaceholderRow[] = [
  { method: 'GET' },
  { method: 'POST' },
  { method: 'PUT' },
  { ghost: true, method: 'DELETE' },
];

/**
 * The palette family a row's tint and chevron are drawn from — the same one
 * its method chip uses, so the row reads as one colour rather than two.
 *
 * `methodColor` can return `'default'`, which is a Chip variant rather than a
 * palette entry, so that case falls back to the neutral text colour.
 */
const toneFor = (theme: Theme, method: string): string => {
  const tone: ChipColor = methodColor(method);
  return tone === 'default' ? theme.palette.text.primary : theme.palette[tone].main;
};

/** Bounded so the copy underneath stays on two lines at the pane's width. */
const CONTENT_MAX_WIDTH = 320;

/** Widths of the two bars standing in for a path and a summary. */
const BAR_SHORT_WIDTH = '22%';
const BAR_LONG_WIDTH = '56%';

/**
 * One of the two bars standing in for a row's text. Paper-coloured rather than
 * tinted, so it reads as an empty slot cut out of the row instead of a second
 * piece of content.
 */
const barSx = (width: string) => (theme: Theme) => ({
  bgcolor: alpha(theme.palette.background.paper, 0.85),
  borderRadius: 999,
  flexShrink: 0,
  height: 6,
  width,
});

export type ResourcePreviewPlaceholderProps = {
  /** Overrides the default explanation under the title. */
  description?: string;
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
  testId = 'resource-preview-placeholder',
  title,
}: ResourcePreviewPlaceholderProps) => {
  const intl = useIntl();

  return (
    <Box
      data-testid={testId}
      sx={(theme) => ({
        alignItems: 'center',
        // A wash of the same three families the method chips use, so the
        // surface belongs to the listing sitting on it. Every stop derives from
        // a palette token, so it re-tints itself in dark mode instead of
        // staying a pale smudge.
        backgroundImage: `linear-gradient(180deg, ${alpha(
          theme.palette.success.light,
          0.1,
        )} 0%, ${theme.palette.background.paper} 52%, ${alpha(
          theme.palette.info.light,
          0.12,
        )} 100%)`,
        border: hairline(theme),
        borderColor: 'divider',
        borderRadius: 2,
        display: 'flex',
        justifyContent: 'center',
        // `minHeight` rather than `height`: it fills a short pane, but a tall
        // enough one lets the content set the height instead of clipping it.
        minHeight: '100%',
        // The glows are positioned against this box and bleed past its edges.
        overflow: 'hidden',
        p: { sm: 3, xs: 2.25 },
        position: 'relative',
      })}
    >
      <Box
        aria-hidden
        sx={(theme) => ({
          ...ambientGlowSx,
          bgcolor: alpha(theme.palette.success.light, 0.34),
          height: 150,
          left: '50%',
          top: theme.spacing(-5),
          transform: 'translateX(-50%)',
          width: 300,
        })}
      />
      <Box
        aria-hidden
        sx={(theme) => ({
          ...ambientGlowSx,
          bgcolor: alpha(theme.palette.warning.light, 0.24),
          height: 130,
          left: '50%',
          top: '42%',
          transform: 'translate(-50%, -50%)',
          width: 220,
        })}
      />
      <Box
        aria-hidden
        sx={(theme) => ({
          ...ambientGlowSx,
          bgcolor: alpha(theme.palette.info.light, 0.36),
          bottom: theme.spacing(-6),
          height: 180,
          right: theme.spacing(-5),
          width: 220,
        })}
      />

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
        <Stack aria-hidden spacing={1.2} sx={{ mb: { sm: 5.5, xs: 4.5 }, width: '100%' }}>
          {PLACEHOLDER_ROWS.map((row) => (
            <Stack
              direction="row"
              key={row.method}
              spacing={1.2}
              sx={(theme) => {
                const tone = toneFor(theme, row.method);

                return {
                  alignItems: 'center',
                  bgcolor: alpha(tone, 0.1),
                  border: hairline(theme),
                  borderColor: alpha(tone, 0.22),
                  borderRadius: 1.25,
                  boxShadow: row.ghost ? 'none' : theme.shadows[1],
                  minHeight: { sm: 40, xs: 38 },
                  px: 1.35,
                  py: 0.9,
                  // The tail of the list, trailing off rather than ending.
                  ...(row.ghost && { opacity: 0.35, width: '86%' }),
                };
              }}
            >
              <Chip
                color={methodColor(row.method)}
                label={row.method}
                size="small"
                sx={{ flexShrink: 0, fontWeight: 700, minWidth: 62 }}
              />
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
                  // wrapper is what colours the glyph.
                  color: alpha(toneFor(theme, row.method), 0.7),
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
