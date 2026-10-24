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

import { Box, Divider, Link, Typography, useTheme, type TypographyProps } from '@wso2/oxygen-ui';
import { useMemo, type ReactNode } from 'react';
import ReactMarkdown, { type Components } from 'react-markdown';
import type { PluggableList } from 'unified';
import { Prism as SyntaxHighlighter } from 'react-syntax-highlighter';
import { oneDark, oneLight } from 'react-syntax-highlighter/dist/esm/styles/prism';
import rehypeRaw from 'rehype-raw';
import rehypeSanitize from 'rehype-sanitize';
import remarkGfm from 'remark-gfm';

import { runtimeConfig } from '@/config/runtime';
import { buildSanitizeSchema } from './sanitizeSchema';

/**
 * Renders a Markdown document as Oxygen typography.
 *
 * Parsing is handled by react-markdown (GFM via remark-gfm). Output is passed
 * through rehype-sanitize with the default GitHub-style allowlist, so raw HTML
 * is restricted to a safe tag/attribute set before it reaches the React tree.
 */

const HEADING_VARIANT: Record<1 | 2 | 3 | 4 | 5 | 6, TypographyProps['variant']> = {
  1: 'h4',
  2: 'subtitle1',
  3: 'h5',
  4: 'h6',
  5: 'subtitle2',
  6: 'subtitle2',
};

/**
 * Semantic heading element. The page title is an h1 and the document title an
 * h2, so the document's own headings start at h3 to keep the outline intact.
 */
const headingElement = (level: 1 | 2 | 3 | 4 | 5 | 6) =>
  `h${Math.min(level + 2, 6)}` as 'h3' | 'h4' | 'h5' | 'h6';

const headingSx = { fontWeight: 'fontWeightBold', mb: 1, mt: 2.5 } as const;

const inlineCodeSx = {
  bgcolor: 'action.hover',
  borderRadius: 0.5,
  fontFamily: 'monospace',
  fontSize: '0.875em',
  px: 0.5,
} as const;

const preBlockSx = {
  bgcolor: 'action.hover',
  borderRadius: 1,
  fontFamily: 'monospace',
  m: 0,
  mb: 2,
  overflowX: 'auto',
  p: 2,
  typography: 'body2',
} as const;

const listSx = { m: 0, mb: 1.5, pl: 3 } as const;

const quoteSx = {
  bgcolor: 'action.hover',
  borderRadius: 1,
  color: 'text.secondary',
  m: 0,
  mb: 2,
  px: 2,
  py: 1.5,
} as const;

const imgBaseSx = {
  display: 'inline-block',
  height: 'auto',
  maxWidth: '100%',
  my: 1.5,
} as const;

// In dark mode, pad embedded images with a white backdrop so logos/badges
// drawn with a transparent background and dark foreground stay readable. The
// same treatment on photo-like images reads as a thin light frame — benign.
const imgDarkSx = {
  ...imgBaseSx,
  backgroundColor: 'common.white',
  borderRadius: 1,
  p: 1,
} as const;

const tableSx = {
  borderCollapse: 'collapse',
  display: 'block',
  mb: 2,
  overflowX: 'auto',
  width: '100%',
} as const;

const tableCellSx = {
  border: 1,
  borderColor: 'divider',
  px: 1.5,
  py: 1,
  typography: 'body2',
} as const;

const tableHeadCellSx = { ...tableCellSx, bgcolor: 'action.hover', fontWeight: 'fontWeightBold' } as const;

const alignToStyle = (align: string | undefined): { textAlign: 'left' | 'center' | 'right' | 'justify' } | undefined => {
  if (align === 'left' || align === 'center' || align === 'right' || align === 'justify') {
    return { textAlign: align };
  }
  return undefined;
};

const renderHeading = (level: 1 | 2 | 3 | 4 | 5 | 6) =>
  function Heading({ children, align }: { children?: ReactNode; align?: string }) {
    return (
      <Typography
        component={headingElement(level)}
        style={alignToStyle(align)}
        sx={headingSx}
        variant={HEADING_VARIANT[level]}
      >
        {children}
      </Typography>
    );
  };

/** Prism's style objects are plain CSSProperties maps keyed by token selector. */
const highlighterCustomStyle = { borderRadius: 8, fontSize: '0.8125rem', margin: 0, marginBottom: 16, padding: 16 };
const highlighterCodeTagProps = { style: { fontFamily: 'inherit' } };

const useMarkdownComponents = (): Components => {
  const theme = useTheme();
  const isDark = theme.palette.mode === 'dark';

  return useMemo(
    () => ({
      h1: renderHeading(1),
      h2: renderHeading(2),
      h3: renderHeading(3),
      h4: renderHeading(4),
      h5: renderHeading(5),
      h6: renderHeading(6),
      p: ({ children, ...rest }) => (
        <Typography
          style={alignToStyle((rest as { align?: string }).align)}
          sx={{ mb: 1.5 }}
          variant="body2"
        >
          {children}
        </Typography>
      ),
      a: ({ href, children }) => (
        <Link href={href} rel="noopener noreferrer" target="_blank">
          {children}
        </Link>
      ),
      ul: ({ children }) => (
        <Box component="ul" sx={listSx}>
          {children}
        </Box>
      ),
      ol: ({ children, start }) => (
        <Box component="ol" start={start} sx={listSx}>
          {children}
        </Box>
      ),
      li: ({ children }) => (
        <Typography component="li" sx={{ mb: 0.5 }} variant="body2">
          {children}
        </Typography>
      ),
      blockquote: ({ children }) => (
        <Box component="blockquote" sx={quoteSx}>
          {children}
        </Box>
      ),
      hr: () => <Divider sx={{ my: 2 }} />,
      // The default `<pre>` wrapper is unwrapped — the SyntaxHighlighter and
      // the untagged-block fallback below each render their own block wrapper.
      pre: ({ children }) => <>{children}</>,
      // Block code with a `language-xxx` class → syntax-highlighted via Prism.
      // Block code without a class (fenced ``` with no language) → plain `<pre>`.
      // Inline `<code>` → inline background pill. react-markdown v10 no longer
      // exposes an `inline` prop, so the two block shapes are detected by
      // either the className or an embedded newline. `node` is pulled out and
      // discarded so it doesn't reach the inline `<Box>` below as a `node`
      // DOM attribute (Oxygen/MUI Box doesn't filter it out).
      // eslint-disable-next-line @typescript-eslint/no-unused-vars
      code: ({ className, children, node: _node, ...rest }) => {
        const text = typeof children === 'string' ? children : String(children ?? '');
        const languageMatch = /language-(\w+)/.exec(className ?? '');
        if (languageMatch) {
          return (
            <SyntaxHighlighter
              PreTag="div"
              codeTagProps={highlighterCodeTagProps}
              customStyle={highlighterCustomStyle}
              language={languageMatch[1]}
              style={isDark ? oneDark : oneLight}
            >
              {text.replace(/\n$/, '')}
            </SyntaxHighlighter>
          );
        }
        if (text.includes('\n')) {
          return (
            <Box component="pre" sx={preBlockSx}>
              <Box component="code">{text.replace(/\n$/, '')}</Box>
            </Box>
          );
        }
        return (
          <Box component="code" sx={inlineCodeSx} {...rest}>
            {children}
          </Box>
        );
      },
      img: ({ src, alt, title, width, height }) => (
        <Box
          alt={alt ?? ''}
          component="img"
          height={height}
          src={src}
          sx={isDark ? imgDarkSx : imgBaseSx}
          title={title}
          width={width}
        />
      ),
      table: ({ children }) => (
        <Box component="table" sx={tableSx}>
          {children}
        </Box>
      ),
      thead: ({ children }) => <Box component="thead">{children}</Box>,
      tbody: ({ children }) => <Box component="tbody">{children}</Box>,
      tr: ({ children }) => <Box component="tr">{children}</Box>,
      // GFM alignment syntax `|:---:|` is carried as a `style={{ textAlign }}`
      // React prop on each `th`/`td` — forward it so aligned columns don't all
      // fall back to left-align.
      th: ({ children, style }) => (
        <Box component="th" style={style} sx={tableHeadCellSx}>
          {children}
        </Box>
      ),
      td: ({ children, style }) => (
        <Box component="td" style={style} sx={tableCellSx}>
          {children}
        </Box>
      ),
    }),
    [isDark],
  );
};

const remarkPlugins = [remarkGfm];
// Order matters: rehype-raw parses the raw HTML strings in the markdown
// source into real hast nodes first; rehype-sanitize then filters the whole
// tree through the allowlist. Reversing the order leaves raw HTML untouched
// by the sanitizer. The schema is built from `defaultSchema` plus any
// operator additions declared under `[api_control_plane.sanitizeHtmlDocs]`,
// so a deployment can widen (never narrow) the allowlist.
const rehypePlugins: PluggableList = runtimeConfig.sanitizeHtmlDocsEnabled
  ? [
      rehypeRaw,
      [
        rehypeSanitize,
        buildSanitizeSchema(
          runtimeConfig.sanitizeHtmlDocsAdditionalAllowedTags,
          runtimeConfig.sanitizeHtmlDocsAdditionalAllowedAttributes,
        ),
      ],
    ]
  : [rehypeRaw];

export type MarkdownViewProps = {
  /** Markdown source. */
  source: string;
  /** Rendered when the source has no content. */
  emptyFallback?: ReactNode;
};

export function MarkdownView({ source, emptyFallback = null }: MarkdownViewProps) {
  const components = useMarkdownComponents();
  if (!source.trim()) return <>{emptyFallback}</>;
  return (
    <Box
      sx={{
        '& > :first-of-type': { mt: 0 },
        '& > :last-child': { mb: 0 },
        minWidth: 0,
        overflowWrap: 'anywhere',
      }}
    >
      <ReactMarkdown components={components} rehypePlugins={rehypePlugins} remarkPlugins={remarkPlugins}>
        {source}
      </ReactMarkdown>
    </Box>
  );
}
