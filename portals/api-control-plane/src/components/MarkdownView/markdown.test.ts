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

import { parseInline, parseMarkdown, safeHref } from './markdown';

describe('parseMarkdown', () => {
  it('parses headings, paragraphs and rules', () => {
    expect(parseMarkdown('# Title\n\nFirst line\nsecond line\n\n---')).toEqual([
      { kind: 'heading', level: 1, children: [{ kind: 'text', text: 'Title' }] },
      { kind: 'paragraph', children: [{ kind: 'text', text: 'First line second line' }] },
      { kind: 'rule' },
    ]);
  });

  it('keeps fenced code verbatim, including Markdown-looking lines', () => {
    expect(parseMarkdown('```bash\n# not a heading\ncurl -X GET\n```')).toEqual([
      { kind: 'code', language: 'bash', text: '# not a heading\ncurl -X GET' },
    ]);
  });

  it('parses bulleted and numbered lists', () => {
    const [bullets, numbers] = parseMarkdown('- one\n- two\n\n3. three\n4. four');
    expect(bullets).toMatchObject({ kind: 'list', ordered: false, items: [[{ text: 'one' }], [{ text: 'two' }]] });
    expect(numbers).toMatchObject({ kind: 'list', ordered: true, start: 3 });
  });

  it('parses block quotes recursively', () => {
    expect(parseMarkdown('> quoted **text**')).toEqual([
      {
        kind: 'quote',
        children: [
          {
            kind: 'paragraph',
            children: [
              { kind: 'text', text: 'quoted ' },
              { kind: 'strong', children: [{ kind: 'text', text: 'text' }] },
            ],
          },
        ],
      },
    ]);
  });

  it('leaves raw HTML as plain text', () => {
    expect(parseMarkdown('<script>alert(1)</script>')).toEqual([
      { kind: 'paragraph', children: [{ kind: 'text', text: '<script>alert(1)</script>' }] },
    ]);
  });

  it('returns no blocks for blank input', () => {
    expect(parseMarkdown('  \n\n ')).toEqual([]);
  });
});

describe('parseInline', () => {
  it('parses code, emphasis and links', () => {
    expect(parseInline('Send `GET` to *the* [docs](https://example.com)')).toEqual([
      { kind: 'text', text: 'Send ' },
      { kind: 'code', text: 'GET' },
      { kind: 'text', text: ' to ' },
      { kind: 'em', children: [{ kind: 'text', text: 'the' }] },
      { kind: 'text', text: ' ' },
      { kind: 'link', href: 'https://example.com', children: [{ kind: 'text', text: 'docs' }] },
    ]);
  });

  it('does not treat snake_case as emphasis', () => {
    expect(parseInline('use reading_list_api here')).toEqual([
      { kind: 'text', text: 'use reading_list_api here' },
    ]);
  });

  it('drops an unsafe link target but keeps its text', () => {
    expect(parseInline('[click](javascript:alert%281%29)')).toEqual([{ kind: 'text', text: 'click' }]);
  });
});

describe('safeHref', () => {
  it.each(['https://example.com', 'http://example.com', 'mailto:team@example.com', '/relative', '#anchor'])(
    'allows %s',
    (href) => {
      expect(safeHref(href)).toBe(href);
    }
  );

  it.each(['javascript:alert(1)', 'JAVA\tSCRIPT:alert(1)', 'data:text/html,x', 'vbscript:x', '//evil.example'])(
    'rejects %s',
    (href) => {
      expect(safeHref(href)).toBeUndefined();
    }
  );
});
