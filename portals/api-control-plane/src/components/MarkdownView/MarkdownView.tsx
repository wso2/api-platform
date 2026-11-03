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
import { useMemo, type ComponentPropsWithoutRef, type CSSProperties, type MouseEvent, type ReactNode } from 'react';
import ReactMarkdown, { type Components, type Options } from 'react-markdown';
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

type MarkdownElementProps = { align?: string; node?: unknown; ref?: unknown; style?: CSSProperties };

const passThrough = <T extends MarkdownElementProps>(props: T) => {
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  const { align: _align, node: _node, ref: _ref, ...rest } = props;
  return rest;
};

const mergeAlignStyle = (style: CSSProperties | undefined, align: string | undefined) => {
  const aligned = alignToStyle(align);
  return style || aligned ? { ...style, ...aligned } : undefined;
};

/** The sanitizer's DOM-clobbering prefix (`defaultSchema.clobberPrefix`) for `id`s. */
const CLOBBER_PREFIX = 'user-content-';

/**
 * Same-page `#fragment` links scroll to their target in place. Target ids are
 * prefixed by the sanitizer, so `#x` is resolved to `user-content-x` first,
 * then `x` (sanitization off). The URL is left alone so the router isn't involved.
 */
const scrollToFragment = (event: MouseEvent<HTMLAnchorElement>, href: string) => {
  let id: string;
  try {
    id = decodeURIComponent(href.slice(1));
  } catch {
    id = href.slice(1);
  }
  const target = document.getElementById(`${CLOBBER_PREFIX}${id}`) ?? document.getElementById(id);
  if (!target) return;
  event.preventDefault();
  target.scrollIntoView({ block: 'start' });
};

const renderHeading = (level: 1 | 2 | 3 | 4 | 5 | 6) =>
  function Heading(props: ComponentPropsWithoutRef<'h1'> & MarkdownElementProps) {
    const { children, ...rest } = passThrough(props);
    return (
      <Typography
        {...rest}
        component={headingElement(level)}
        style={mergeAlignStyle(props.style, props.align)}
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
      p: (props: ComponentPropsWithoutRef<'p'> & MarkdownElementProps) => {
        const { children, ...rest } = passThrough(props);
        return (
          <Typography {...rest} style={mergeAlignStyle(props.style, props.align)} sx={{ mb: 1.5 }} variant="body2">
            {children}
          </Typography>
        );
      },
      a: (props) => {
        const { children, ...rest } = passThrough(props);
        const href = typeof rest.href === 'string' ? rest.href : '';
        if (href.startsWith('#')) {
          return (
            <Link {...rest} onClick={(event) => scrollToFragment(event, href)}>
              {children}
            </Link>
          );
        }
        // Set after the spread so a document can't override them.
        return (
          <Link {...rest} rel="noopener noreferrer" target="_blank">
            {children}
          </Link>
        );
      },
      ul: (props) => {
        const { children, ...rest } = passThrough(props);
        return (
          <Box {...rest} component="ul" sx={listSx}>
            {children}
          </Box>
        );
      },
      ol: (props) => {
        const { children, ...rest } = passThrough(props);
        return (
          <Box {...rest} component="ol" sx={listSx}>
            {children}
          </Box>
        );
      },
      li: (props) => {
        const { children, ...rest } = passThrough(props);
        return (
          <Typography {...rest} component="li" sx={{ mb: 0.5 }} variant="body2">
            {children}
          </Typography>
        );
      },
      blockquote: (props) => {
        const { children, ...rest } = passThrough(props);
        return (
          <Box {...rest} component="blockquote" sx={quoteSx}>
            {children}
          </Box>
        );
      },
      hr: (props) => <Divider {...passThrough(props)} sx={{ my: 2 }} />,
      // The default `<pre>` wrapper is unwrapped — the SyntaxHighlighter and
      // the untagged-block fallback below each render their own block wrapper.
      pre: ({ children }) => <>{children}</>,
      // Block code with a `language-xxx` class → syntax-highlighted via Prism.
      // Block code without a class (fenced ``` with no language) → plain `<pre>`.
      // Inline `<code>` → inline background pill. react-markdown v10 no longer
      // exposes an `inline` prop, so the two block shapes are detected by
      // either the className or an embedded newline.
      code: (props) => {
        const { className, children, ...rest } = passThrough(props);
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
          <Box {...rest} className={className} component="code" sx={inlineCodeSx}>
            {children}
          </Box>
        );
      },
      img: (props) => {
        const { alt, ...rest } = passThrough(props);
        return <Box {...rest} alt={alt ?? ''} component="img" sx={isDark ? imgDarkSx : imgBaseSx} />;
      },
      table: (props) => {
        const { children, ...rest } = passThrough(props);
        return (
          <Box {...rest} component="table" sx={tableSx}>
            {children}
          </Box>
        );
      },
      thead: (props) => <Box {...passThrough(props)} component="thead" />,
      tbody: (props) => <Box {...passThrough(props)} component="tbody" />,
      tr: (props) => <Box {...passThrough(props)} component="tr" />,
      // GFM column alignment arrives as `style`, raw-HTML cells as `align`.
      th: (props: ComponentPropsWithoutRef<'th'> & MarkdownElementProps) => (
        <Box
          {...passThrough(props)}
          component="th"
          style={mergeAlignStyle(props.style, props.align)}
          sx={tableHeadCellSx}
        />
      ),
      td: (props: ComponentPropsWithoutRef<'td'> & MarkdownElementProps) => (
        <Box {...passThrough(props)} component="td" style={mergeAlignStyle(props.style, props.align)} sx={tableCellSx} />
      ),
    }),
    [isDark],
  );
};

const remarkPlugins = [remarkGfm];
// With sanitization on, the sanitizer adds the clobber prefix to ids; turning
// off remark-rehype's own prefix keeps footnote ids from being prefixed twice.
const remarkRehypeOptions: Options['remarkRehypeOptions'] = runtimeConfig.sanitizeHtmlDocsEnabled
  ? { clobberPrefix: '' }
  : undefined;
// Order matters: rehype-raw parses the raw HTML strings in the markdown
// source into real hast nodes first; rehype-sanitize then filters the whole
// tree through the allowlist (`defaultSchema` plus any configured additions).
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
      <ReactMarkdown
        components={components}
        rehypePlugins={rehypePlugins}
        remarkPlugins={remarkPlugins}
        remarkRehypeOptions={remarkRehypeOptions}
      >
        {source}
      </ReactMarkdown>
    </Box>
  );
}
