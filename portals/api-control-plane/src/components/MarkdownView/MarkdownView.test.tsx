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

import { describe, expect, it } from 'vitest';

import { renderWithProviders, screen } from '@/test/utils';
import { MarkdownView } from './MarkdownView';

describe('MarkdownView', () => {
  it('renders document structure as elements', () => {
    renderWithProviders(<MarkdownView source={'# Guide\n\nRead the [docs](https://example.com).\n\n- step'} />);

    expect(screen.getByRole('heading', { name: 'Guide' })).toBeInTheDocument();
    const link = screen.getByRole('link', { name: 'docs' });
    expect(link).toHaveAttribute('href', 'https://example.com');
    expect(link).toHaveAttribute('rel', 'noopener noreferrer');
    expect(link).toHaveAttribute('target', '_blank');
    expect(screen.getByRole('listitem')).toHaveTextContent('step');
  });

  it('renders a markdown image with alt text', () => {
    const { container } = renderWithProviders(
      <MarkdownView source={'![logo](https://example.com/logo.png "WSO2")'} />
    );

    const img = container.querySelector('img');
    expect(img).not.toBeNull();
    expect(img).toHaveAttribute('src', 'https://example.com/logo.png');
    expect(img).toHaveAttribute('alt', 'logo');
    expect(img).toHaveAttribute('title', 'WSO2');
  });

  it('honors align="center" on raw HTML <p> and <h*> and renders centered images inline-block', () => {
    const { container } = renderWithProviders(
      <MarkdownView
        source={[
          '<div align="center">',
          '  <img src="https://example.com/logo.png" alt="logo"/>',
          '  <h1 align="center">Title</h1>',
          '</div>',
          '',
          '<p align="center"><strong>English</strong></p>',
        ].join('\n')}
      />
    );

    // `align="center"` on <div> survives sanitize so browsers can apply the
    // UA stylesheet. (Tested indirectly: the attribute reaches the DOM.)
    expect(container.querySelector('div[align="center"]')).not.toBeNull();

    // <p align="center"> is replaced by Typography; the align must survive as
    // an inline textAlign style.
    const paragraph = container.querySelector('p');
    expect(paragraph).toHaveStyle({ textAlign: 'center' });

    // Same for <h1 align="center"> → heading override.
    const heading = container.querySelector('h3'); // h1 → h3 semantic shift
    expect(heading).toHaveStyle({ textAlign: 'center' });

    // The image must be inline-block so parent text-align: center can center
    // it; a block image would ignore text-align entirely.
    const img = container.querySelector('img');
    expect(img).toHaveStyle({ display: 'inline-block' });
  });

  it('renders raw HTML <img>/<div>/<p> embedded in markdown source', () => {
    // Shape taken from the Spec Kit README — a <div> wrapper, an <img> inside,
    // and a <p align="center"> paragraph. Without rehype-raw these would all
    // be dropped as unparsed text.
    const source = [
      '<div align="center">',
      '  <img src="https://example.com/hero.webp" alt="hero" width="200" height="200"/>',
      '  <h1>Title</h1>',
      '</div>',
      '',
      '<p align="center"><strong>English</strong> · <a href="./other.md">Other</a></p>',
    ].join('\n');
    const { container } = renderWithProviders(<MarkdownView source={source} />);

    const img = container.querySelector('img');
    expect(img).not.toBeNull();
    expect(img).toHaveAttribute('src', 'https://example.com/hero.webp');
    expect(img).toHaveAttribute('alt', 'hero');
    expect(container.querySelector('div[align="center"]')).not.toBeNull();
    expect(screen.getByText('Title')).toBeInTheDocument();
    expect(screen.getByText('English')).toBeInTheDocument();
  });

  it('honors GFM column alignment on headers and cells', () => {
    const { container } = renderWithProviders(
      <MarkdownView source={'| L | C | R |\n|:---|:---:|---:|\n| a | b | c |'} />
    );

    const headers = container.querySelectorAll('th');
    expect(headers[0]).toHaveStyle({ textAlign: 'left' });
    expect(headers[1]).toHaveStyle({ textAlign: 'center' });
    expect(headers[2]).toHaveStyle({ textAlign: 'right' });

    const cells = container.querySelectorAll('td');
    expect(cells[0]).toHaveStyle({ textAlign: 'left' });
    expect(cells[1]).toHaveStyle({ textAlign: 'center' });
    expect(cells[2]).toHaveStyle({ textAlign: 'right' });
  });

  it('renders a GFM table', () => {
    renderWithProviders(
      <MarkdownView source={'| Name | Role |\n| --- | --- |\n| Alice | Admin |\n| Bob | Reader |'} />
    );

    expect(screen.getByRole('table')).toBeInTheDocument();
    expect(screen.getByRole('columnheader', { name: 'Name' })).toBeInTheDocument();
    expect(screen.getByRole('columnheader', { name: 'Role' })).toBeInTheDocument();
    expect(screen.getByRole('cell', { name: 'Alice' })).toBeInTheDocument();
    expect(screen.getByRole('cell', { name: 'Admin' })).toBeInTheDocument();
  });

  it('renders a GFM task list', () => {
    const { container } = renderWithProviders(
      <MarkdownView source={'- [x] Done\n- [ ] Pending'} />
    );

    const checkboxes = container.querySelectorAll('input[type="checkbox"]');
    expect(checkboxes).toHaveLength(2);
    expect(checkboxes[0]).toBeChecked();
    expect(checkboxes[1]).not.toBeChecked();
  });

  it('renders GFM strikethrough', () => {
    const { container } = renderWithProviders(<MarkdownView source="This is ~~gone~~ text." />);

    expect(container.querySelector('del')).toHaveTextContent('gone');
  });

  it('syntax-highlights a fenced code block with a language', () => {
    const { container } = renderWithProviders(
      <MarkdownView source={'```ts\nconst x: number = 1;\n```'} />
    );

    // Prism tokenizes the source into classed spans. A keyword like `const`
    // must have been pulled into its own `.token` span, proving tokenization
    // happened (as opposed to the plain-text fallback below).
    const tokens = container.querySelectorAll('span.token');
    expect(tokens.length).toBeGreaterThan(0);
    const keywordSpan = Array.from(tokens).find((span) => span.textContent === 'const');
    expect(keywordSpan).toBeDefined();
  });

  it('renders a fenced block without a language as plain preformatted text', () => {
    const { container } = renderWithProviders(
      <MarkdownView source={'```\nplain block\n```'} />
    );

    const pre = container.querySelector('pre');
    expect(pre).not.toBeNull();
    expect(container.querySelector('.token.keyword')).toBeNull();
    expect(pre).toHaveTextContent('plain block');
  });

  it('renders inline code without syntax highlighting', () => {
    renderWithProviders(<MarkdownView source={'Call `fetch()` with a URL.'} />);

    expect(screen.getByText('fetch()')).toBeInTheDocument();
  });

  it('strips disallowed HTML — no script executes and no event-handler attribute survives', () => {
    const { container } = renderWithProviders(
      <MarkdownView
        source={'<script>window.pwned=1</script>\n\n<a href="https://example.com" onclick="pwned()">link</a>'}
      />
    );

    expect(container.querySelector('script')).toBeNull();
    expect(container.innerHTML).not.toContain('onclick');
    expect(container.innerHTML).not.toContain('pwned');
  });

  it('strips javascript: URLs on links', () => {
    const { container } = renderWithProviders(
      // eslint-disable-next-line no-script-url
      <MarkdownView source={'[click](javascript:alert(1))'} />
    );

    const link = container.querySelector('a');
    // react-markdown's defaultUrlTransform drops disallowed URLs; the anchor
    // renders without an href rather than carrying a javascript: scheme.
    expect(link?.getAttribute('href') ?? '').not.toMatch(/^javascript:/i);
  });

  it('denies a tag that is not on the default allowlist and has no config addition', () => {
    const { container } = renderWithProviders(
      <MarkdownView source={'<figure><figcaption>caption</figcaption></figure>'} />
    );

    expect(container.querySelector('figure')).toBeNull();
    expect(container.querySelector('figcaption')).toBeNull();
  });

  it('renders the fallback for an empty document', () => {
    renderWithProviders(<MarkdownView emptyFallback={<p>Nothing here</p>} source="" />);

    expect(screen.getByText('Nothing here')).toBeInTheDocument();
  });
});
