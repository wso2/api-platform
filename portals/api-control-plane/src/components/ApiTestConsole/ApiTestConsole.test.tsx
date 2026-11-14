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

import { describe, expect, it, vi } from 'vitest';

import { renderWithProviders, screen, waitFor } from '@/test/utils';
import { ApiTestConsole } from './ApiTestConsole';
import {
  ConsoleTransportError,
  type ConsoleSendRequest,
  type ConsoleSendResult,
  type ConsoleTransport,
} from './types';

const SPEC = {
  paths: {
    '/books': {
      get: {
        parameters: [
          { in: 'query', name: 'limit', schema: { maximum: 100, minimum: 1, type: 'integer' } },
        ],
        responses: {
          '200': {
            content: { 'application/json': { example: { books: [] } } },
            description: 'the list',
          },
        },
        summary: 'List books',
      },
      post: {
        requestBody: {
          content: {
            'application/json': {
              schema: {
                properties: { title: { example: 'Dune', type: 'string' } },
                type: 'object',
              },
            },
          },
          required: true,
        },
        responses: { '201': { description: 'created' } },
        summary: 'Add a book',
      },
    },
    '/books/{id}': {
      get: {
        parameters: [{ in: 'path', name: 'id', required: true, schema: { type: 'string' } }],
        responses: { '404': { description: 'missing' } },
        summary: 'Get a book',
      },
    },
  },
};

const result = (overrides: Partial<ConsoleSendResult> = {}): ConsoleSendResult => ({
  binary: false,
  bodyText: '{"books":[]}',
  durationMs: 12,
  headers: [['content-type', 'application/json']],
  sizeBytes: 12,
  status: 200,
  statusText: 'OK',
  truncated: false,
  url: 'https://gw.example.com/p/books',
  ...overrides,
});

const renderConsole = (
  transport: ConsoleTransport,
  props: Partial<React.ComponentProps<typeof ApiTestConsole>> = {},
) =>
  renderWithProviders(
    <ApiTestConsole
      baseUrl="https://gw.example.com/p"
      spec={SPEC}
      transport={transport}
      {...props}
    />,
  );

const sendButton = () => screen.getByRole('button', { name: /^Send/ });

describe('ApiTestConsole — browsing', () => {
  it('lists the operations and opens the first', () => {
    renderConsole(vi.fn());

    expect(screen.getByRole('heading', { name: 'List books' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Add a book/ })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Get a book/ })).toBeInTheDocument();
  });

  it('opens another operation, and narrows the list by search', async () => {
    const { user } = renderConsole(vi.fn());

    await user.click(screen.getByRole('button', { name: /Get a book/ }));
    expect(screen.getByRole('heading', { name: 'Get a book' })).toBeInTheDocument();

    await user.type(screen.getByRole('textbox', { name: 'Search endpoints' }), 'add');
    expect(screen.queryByRole('button', { name: /Get a book/ })).toBeNull();
    expect(screen.getByRole('button', { name: /Add a book/ })).toBeInTheDocument();
  });

  it('shows the documented responses before anything is sent', () => {
    renderConsole(vi.fn());

    expect(screen.getByText(/Possible responses/i)).toBeInTheDocument();
    expect(screen.getByText('the list')).toBeInTheDocument();
  });

  it('says so when the definition has nothing to test', () => {
    renderWithProviders(
      <ApiTestConsole
        baseUrl="https://gw.example.com/p"
        spec={{ paths: {} }}
        transport={vi.fn()}
      />,
    );

    expect(screen.getByText(/no operations to test/i)).toBeInTheDocument();
  });
});

describe('ApiTestConsole — sending', () => {
  it('sends the request and shows the response', async () => {
    const transport = vi.fn<ConsoleTransport>().mockResolvedValue(result());
    const { user } = renderConsole(transport);

    await user.click(sendButton());

    expect(await screen.findByText('200 OK')).toBeInTheDocument();
    expect(screen.getByRole('region', { name: 'Response body' })).toHaveTextContent('"books"');
    const [sent] = transport.mock.calls[0];
    expect(sent).toMatchObject({
      baseUrl: 'https://gw.example.com/p',
      method: 'GET',
      path: '/books',
      query: [],
    });
  });

  it('will not send while something is wrong, and says what', async () => {
    const transport = vi.fn<ConsoleTransport>().mockResolvedValue(result());
    const { user } = renderConsole(transport);
    await user.click(screen.getByRole('button', { name: /Get a book/ }));

    await user.click(sendButton());

    expect(await screen.findByText('Required')).toBeInTheDocument();
    expect(transport).not.toHaveBeenCalled();
  });

  it('encodes a path value so it stays one segment', async () => {
    const transport = vi
      .fn<ConsoleTransport>()
      .mockResolvedValue(result({ status: 404, statusText: 'Not Found' }));
    const { user } = renderConsole(transport);
    await user.click(screen.getByRole('button', { name: /Get a book/ }));

    await user.type(screen.getByRole('textbox', { name: 'id' }), '../admin');
    await user.click(sendButton());

    await waitFor(() => expect(transport).toHaveBeenCalled());
    expect(transport.mock.calls[0][0].path).toBe('/books/..%2Fadmin');
  });

  it('checks a number before sending it', async () => {
    const transport = vi.fn<ConsoleTransport>();
    const { user } = renderConsole(transport);

    await user.type(screen.getByRole('textbox', { name: 'limit' }), 'abc');
    await user.click(sendButton());

    expect(await screen.findByText('Must be a whole number')).toBeInTheDocument();
    expect(transport).not.toHaveBeenCalled();
  });

  it('starts an operation’s body from its example, and refuses malformed JSON', async () => {
    const transport = vi
      .fn<ConsoleTransport>()
      .mockResolvedValue(result({ status: 201, statusText: 'Created' }));
    const { user } = renderConsole(transport);
    await user.click(screen.getByRole('button', { name: /Add a book/ }));
    await user.click(screen.getByRole('tab', { name: /Body/ }));

    const editor = screen.getByRole('textbox', { name: 'Request body' });
    expect(editor).toHaveValue('{\n  "title": "Dune"\n}');

    await user.clear(editor);
    await user.type(editor, 'not json');
    await user.click(sendButton());
    expect((await screen.findAllByText(/not valid JSON/i)).length).toBeGreaterThan(0);
    expect(transport).not.toHaveBeenCalled();

    await user.click(screen.getByRole('button', { name: 'Reset to example' }));
    await user.click(sendButton());
    expect(await screen.findByText('201 Created')).toBeInTheDocument();
    expect(transport.mock.calls[0][0].body).toMatchObject({
      kind: 'text',
      mediaType: 'application/json',
    });
  });

  it('refuses a header the browser owns, without sending', async () => {
    const transport = vi.fn<ConsoleTransport>();
    const { user } = renderConsole(transport);
    await user.click(screen.getByRole('tab', { name: /Headers/ }));

    await user.click(screen.getByRole('button', { name: /Add header/ }));
    // An Accept row is pre-filled from the documented response, so the new row is the last.
    await user.type(
      screen.getAllByRole('textbox', { name: 'Header name' }).at(-1) as HTMLElement,
      'Host',
    );
    await user.click(sendButton());

    expect(await screen.findByText(/cannot be sent from here/i)).toBeInTheDocument();
    expect(transport).not.toHaveBeenCalled();
  });
});

describe('ApiTestConsole — untrusted responses', () => {
  it('shows markup in a response as text', async () => {
    const hostile = '{"a":"<img src=x onerror=alert(1)>","b":"<script>alert(2)</script>"}';
    const transport = vi.fn<ConsoleTransport>().mockResolvedValue(result({ bodyText: hostile }));
    const { user, container } = renderConsole(transport);

    await user.click(sendButton());

    await screen.findByText('200 OK');
    expect(container.querySelector('img')).toBeNull();
    expect(container.querySelector('script')).toBeNull();
    expect(screen.getByRole('region', { name: 'Response body' })).toHaveTextContent(
      '<img src=x onerror=alert(1)>',
    );
  });

  it('does not show a binary body, and says it was cut when it is', async () => {
    const transport = vi
      .fn<ConsoleTransport>()
      .mockResolvedValueOnce(result({ binary: true, bodyText: '' }))
      .mockResolvedValueOnce(result({ truncated: true }));
    const { user } = renderConsole(transport);

    await user.click(sendButton());
    expect(await screen.findByText(/not text, so it is not shown/i)).toBeInTheDocument();

    await user.click(sendButton());
    expect(await screen.findByText(/cut off/i)).toBeInTheDocument();
  });

  it('shows a transport’s own message, but never an unknown error’s text', async () => {
    const transport = vi
      .fn<ConsoleTransport>()
      .mockRejectedValueOnce(new ConsoleTransportError('The gateway did not respond in time.'))
      .mockRejectedValueOnce(new Error('internal: bearer abc123 at https://secret.example.com'));
    const { user } = renderConsole(transport);

    await user.click(sendButton());
    expect(await screen.findByText('The gateway did not respond in time.')).toBeInTheDocument();

    await user.click(sendButton());
    expect(await screen.findByText(/could not be sent. Try again/i)).toBeInTheDocument();
    expect(screen.queryByText(/abc123/)).toBeNull();
    expect(screen.queryByText(/secret.example.com/)).toBeNull();
  });
});

describe('ApiTestConsole — cancelling', () => {
  it('stops a request in flight and shows nothing for it', async () => {
    let signal: AbortSignal | undefined;
    const transport = vi.fn<ConsoleTransport>().mockImplementation(
      (_request, s) =>
        new Promise((_resolve, reject) => {
          signal = s;
          s.addEventListener('abort', () =>
            reject(new ConsoleTransportError('', { aborted: true })),
          );
        }),
    );
    const { user } = renderConsole(transport);

    await user.click(sendButton());
    await screen.findByText(/Waiting for the gateway/i);
    await user.click(screen.getByRole('button', { name: /^Cancel/ }));

    await waitFor(() => expect(signal?.aborted).toBe(true));
    expect(await screen.findByText(/Possible responses/i)).toBeInTheDocument();
    expect(screen.queryByRole('alert')).toBeNull();
  });

  it('aborts what is in flight when it goes away', async () => {
    let signal: AbortSignal | undefined;
    const transport = vi.fn<ConsoleTransport>().mockImplementation(
      (_request, s) =>
        new Promise(() => {
          signal = s;
        }),
    );
    const { user, unmount } = renderConsole(transport);

    await user.click(sendButton());
    await waitFor(() => expect(signal).toBeDefined());
    unmount();

    expect(signal?.aborted).toBe(true);
  });
});

describe('ApiTestConsole — credentials', () => {
  const autoHeaders = [{ name: 'X-API-Key', secret: true, value: 'sekret-value' }];

  it('sends the real value but never shows it', async () => {
    const transport = vi.fn<ConsoleTransport>().mockResolvedValue(result());
    const { user, container } = renderConsole(transport, { autoHeaders });

    await user.click(screen.getByRole('tab', { name: /Headers/ }));
    expect(screen.getByText('<redacted>')).toBeInTheDocument();

    await user.click(sendButton());
    await screen.findByText('200 OK');
    await user.click(screen.getByRole('tab', { name: 'Request' }));

    expect(transport.mock.calls[0][0].headers).toContainEqual(['X-API-Key', 'sekret-value']);
    expect(container).not.toHaveTextContent('sekret-value');
  });

  it('keeps a credential out of the URL it shows', async () => {
    const transport = vi.fn<ConsoleTransport>().mockResolvedValue(result());
    const { container } = renderConsole(transport, {
      autoQuery: [{ name: 'api_key', secret: true, value: 'sekret-value' }],
    });

    expect(container).not.toHaveTextContent('sekret-value');
  });
});

describe('ApiTestConsole — mirroring', () => {
  it('reports the latest valid request so another view can follow it', async () => {
    const onRequestChange = vi.fn<(request: ConsoleSendRequest) => void>();
    const { user } = renderConsole(vi.fn(), { onRequestChange });

    await waitFor(() => expect(onRequestChange).toHaveBeenCalled());
    expect(onRequestChange.mock.calls.at(-1)?.[0]).toMatchObject({ method: 'GET', path: '/books' });

    await user.type(screen.getByRole('textbox', { name: 'limit' }), '5');
    await waitFor(() =>
      expect(onRequestChange.mock.calls.at(-1)?.[0].query).toEqual([['limit', '5']]),
    );
  });
});

describe('ApiTestConsole — response format', () => {
  const MULTI = {
    paths: {
      '/books': {
        get: {
          responses: {
            '200': {
              content: { 'application/json': {}, 'application/xml': {} },
              description: 'ok',
            },
          },
          summary: 'List books',
        },
      },
    },
  };

  it('offers the documented media types and sends the chosen one as Accept', async () => {
    const transport = vi.fn<ConsoleTransport>().mockResolvedValue(result());
    const { user } = renderConsole(transport, { spec: MULTI });

    await user.click(screen.getByRole('tab', { name: /Headers/ }));
    await user.click(screen.getByRole('combobox', { name: /Response format/ }));
    await user.click(screen.getByRole('option', { name: 'application/xml' }));
    await user.click(sendButton());

    await waitFor(() => expect(transport).toHaveBeenCalled());
    expect((transport.mock.calls[0][0] as ConsoleSendRequest).headers).toContainEqual([
      'Accept',
      'application/xml',
    ]);
  });

  it('sends no Accept header when "No preference" is chosen', async () => {
    const transport = vi.fn<ConsoleTransport>().mockResolvedValue(result());
    const { user } = renderConsole(transport, { spec: MULTI });

    await user.click(screen.getByRole('tab', { name: /Headers/ }));
    await user.click(screen.getByRole('combobox', { name: /Response format/ }));
    await user.click(screen.getByRole('option', { name: 'No preference' }));
    await user.click(sendButton());

    await waitFor(() => expect(transport).toHaveBeenCalled());
    const names = (transport.mock.calls[0][0] as ConsoleSendRequest).headers.map(([name]) => name);
    expect(names).not.toContain('Accept');
  });
});

describe('ApiTestConsole — nested schemas', () => {
  const NESTED = {
    components: {
      schemas: {
        Author: {
          properties: {
            address: { $ref: '#/components/schemas/Address' },
            name: { type: 'string' },
          },
          type: 'object',
        },
        Address: {
          properties: { city: { type: 'string' }, owner: { $ref: '#/components/schemas/Author' } },
          type: 'object',
        },
      },
    },
    paths: { '/a': { get: { responses: {}, summary: 'A' } } },
  };

  it('opens a nested object to show its fields, and stops at a cycle', async () => {
    const { user } = renderConsole(vi.fn(), { spec: NESTED });

    await user.click(screen.getByRole('button', { name: /Schemas/ }));
    await user.click(screen.getByRole('button', { name: /Author/ }));

    expect(screen.queryByText('city')).not.toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Show the fields of address' }));
    expect(screen.getByText('city')).toBeInTheDocument();

    // `owner` leads back to Author, which is listed once more and then not unfolded again.
    await user.click(screen.getByRole('button', { name: 'Show the fields of owner' }));
    // Only the first `address` can be opened; the one under `owner` would repeat it.
    expect(screen.queryAllByRole('button', { name: 'Show the fields of address' })).toHaveLength(0);
    expect(screen.getAllByRole('button', { name: 'Hide the fields of address' })).toHaveLength(1);
  });
});
