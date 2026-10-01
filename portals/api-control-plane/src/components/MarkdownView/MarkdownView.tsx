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

import { Box, Divider, Link, Typography, type TypographyProps } from '@wso2/oxygen-ui';
import { Fragment, useMemo, type ReactNode } from 'react';

import { parseMarkdown, type MarkdownBlock, type MarkdownInline } from './markdown';

/**
 * Renders a Markdown document as Oxygen typography.
 *
 * Every node becomes a React element, so document text is always an escaped
 * text node — there is no `dangerouslySetInnerHTML` anywhere on this path, and
 * raw HTML inside a document is displayed as literal text.
 */

/** Visual size per Markdown heading level; the document's own title sits above these. */
const HEADING_VARIANT: Record<number, TypographyProps['variant']> = {
  1: 'h5',
  2: 'h6',
  3: 'subtitle1',
  4: 'subtitle2',
  5: 'subtitle2',
  6: 'subtitle2',
};

/**
 * Semantic heading element. The page title is an h1 and the document title an
 * h2, so the document's own headings start at h3 to keep the outline intact.
 */
const headingElement = (level: number) => `h${Math.min(level + 2, 6)}` as 'h3' | 'h4' | 'h5' | 'h6';

const codeSx = {
  bgcolor: 'action.hover',
  borderRadius: 0.5,
  fontFamily: 'monospace',
  fontSize: '0.875em',
  px: 0.5,
} as const;

function renderInline(nodes: MarkdownInline[]): ReactNode {
  return nodes.map((node, index) => {
    switch (node.kind) {
      case 'text':
        return <Fragment key={index}>{node.text}</Fragment>;
      case 'code':
        return (
          <Box component="code" key={index} sx={codeSx}>
            {node.text}
          </Box>
        );
      case 'strong':
        return (
          <Box component="strong" key={index} sx={{ fontWeight: 'fontWeightBold' }}>
            {renderInline(node.children)}
          </Box>
        );
      case 'em':
        return <em key={index}>{renderInline(node.children)}</em>;
      case 'link':
        return (
          <Link href={node.href} key={index} rel="noopener noreferrer" target="_blank">
            {renderInline(node.children)}
          </Link>
        );
      case 'break':
        return <br key={index} />;
      default:
        return null;
    }
  });
}

function renderBlock(block: MarkdownBlock, index: number): ReactNode {
  switch (block.kind) {
    case 'heading':
      return (
        <Typography
          component={headingElement(block.level)}
          key={index}
          sx={{ fontWeight: 'fontWeightBold', mb: 1, mt: index === 0 ? 0 : 2.5 }}
          variant={HEADING_VARIANT[block.level]}
        >
          {renderInline(block.children)}
        </Typography>
      );
    case 'paragraph':
      return (
        <Typography key={index} sx={{ mb: 1.5 }} variant="body2">
          {renderInline(block.children)}
        </Typography>
      );
    case 'code':
      return (
        <Box
          component="pre"
          key={index}
          sx={{
            bgcolor: 'action.hover',
            borderRadius: 1,
            fontFamily: 'monospace',
            fontSize: '0.8125rem',
            m: 0,
            mb: 2,
            overflowX: 'auto',
            p: 2,
          }}
        >
          <code>{block.text}</code>
        </Box>
      );
    case 'list':
      return (
        <Box
          component={block.ordered ? 'ol' : 'ul'}
          key={index}
          start={block.ordered && block.start !== 1 ? block.start : undefined}
          sx={{ m: 0, mb: 1.5, pl: 3 }}
        >
          {block.items.map((item, itemIndex) => (
            <Typography component="li" key={itemIndex} sx={{ mb: 0.5 }} variant="body2">
              {renderInline(item)}
            </Typography>
          ))}
        </Box>
      );
    case 'quote':
      return (
        <Box
          component="blockquote"
          key={index}
          sx={{ bgcolor: 'action.hover', borderRadius: 1, color: 'text.secondary', m: 0, mb: 2, px: 2, py: 1.5 }}
        >
          {block.children.map(renderBlock)}
        </Box>
      );
    case 'rule':
      return <Divider key={index} sx={{ my: 2 }} />;
    default:
      return null;
  }
}

export type MarkdownViewProps = {
  /** Markdown source. */
  source: string;
  /** Rendered when the source has no content. */
  emptyFallback?: ReactNode;
};

export function MarkdownView({ source, emptyFallback = null }: MarkdownViewProps) {
  const blocks = useMemo(() => parseMarkdown(source), [source]);
  if (blocks.length === 0) return <>{emptyFallback}</>;
  return (
    <Box sx={{ '& > :last-child': { mb: 0 }, minWidth: 0, overflowWrap: 'anywhere' }}>
      {blocks.map(renderBlock)}
    </Box>
  );
}
