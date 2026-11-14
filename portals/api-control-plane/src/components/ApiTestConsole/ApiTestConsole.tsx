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

import { useCallback, useEffect, useMemo, useRef, useState, type KeyboardEvent } from 'react';
import { Alert, Card, CardContent, Grid, Stack } from '@wso2/oxygen-ui';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import { SecondaryText } from '@/components/common/SecondaryText';
import { buildSendRequest, type BuildResult, type RequestDraft } from './request/buildRequest';
import { initialDraft } from './request/draft';
import { displayUrl } from './request/display';
import { parseConsoleSpec } from './spec/parseSpec';
import {
  ConsoleTransportError,
  type ConsoleAutoParam,
  type ConsoleSendRequest,
  type ConsoleTransport,
} from './types';
import { OperationSidebar, type MethodFilter } from './view/OperationSidebar';
import { RequestPanel, type RequestTab } from './view/RequestPanel';
import { ResponsePanel, type Outcome, type ResponseTab } from './view/ResponsePanel';
import { SchemaPage } from './view/SchemaViews';

const messages = defineMessages({
  copyFailed: {
    id: 'apiControlPlane.components.ApiTestConsole.ApiTestConsole.copyFailed',
    defaultMessage: 'Your browser did not allow copying. Select the text and copy it yourself.',
  },
  noOperations: {
    id: 'apiControlPlane.components.ApiTestConsole.ApiTestConsole.noOperations',
    defaultMessage: 'This API’s definition has no operations to test.',
  },
  unexpected: {
    id: 'apiControlPlane.components.ApiTestConsole.ApiTestConsole.unexpected',
    defaultMessage: 'The request could not be sent. Try again.',
  },
});

export type ApiTestConsoleProps = {
  /** The parsed OpenAPI document. Read as data only; nothing in it is executed or fetched. */
  spec: Record<string, unknown>;
  /** The URL operations are called under — a gateway's invoke URL. Not editable. */
  baseUrl: string;
  /** How a request leaves the browser. The console never sends anything itself. */
  transport: ConsoleTransport;
  /** Headers the console adds to every request, e.g. the test key. Shown locked. */
  autoHeaders?: readonly ConsoleAutoParam[];
  /** Query parameters the console adds to every request. Shown locked. */
  autoQuery?: readonly ConsoleAutoParam[];
  /** Short note on how requests are sent, shown beside the response status. */
  routeLabel?: string;
  /** Called with the latest valid request, so another view can mirror it. */
  onRequestChange?: (request: ConsoleSendRequest) => void;
};

/** Saves the API definition as `openapi.json`. A Blob URL to local text, revoked right after the click. */
export const downloadOpenApi = (spec: Record<string, unknown>): void => {
  const url = URL.createObjectURL(
    new Blob([JSON.stringify(spec, null, 2)], { type: 'application/json' }),
  );
  const link = document.createElement('a');
  link.href = url;
  link.download = 'openapi.json';
  link.click();
  // Some browsers start the download after the click returns, so revoke on the next tick.
  window.setTimeout(() => URL.revokeObjectURL(url), 0);
};

const NO_PARAMS: readonly ConsoleAutoParam[] = [];

/** A request without its files' contents, as a comparable string. */
const requestSignature = (request: ConsoleSendRequest): string =>
  JSON.stringify(request, (_key, value) => (value instanceof File ? `file:${value.name}` : value));

/** Copies text; resolves to whether the browser accepted it. */
const writeClipboard = async (text: string): Promise<boolean> => {
  try {
    if (!navigator.clipboard) return false;
    await navigator.clipboard.writeText(text);
    return true;
  } catch {
    return false;
  }
};

export function ApiTestConsole({
  spec,
  baseUrl,
  transport,
  autoHeaders = NO_PARAMS,
  autoQuery = NO_PARAMS,
  routeLabel,
  onRequestChange,
}: ApiTestConsoleProps) {
  const intl = useIntl();
  const model = useMemo(() => parseConsoleSpec(spec), [spec]);

  const [selectedKey, setSelectedKey] = useState<string | undefined>(model.operations[0]?.key);
  const [selectedSchema, setSelectedSchema] = useState<string | undefined>(undefined);
  const [search, setSearch] = useState('');
  const [method, setMethod] = useState<MethodFilter>('all');
  const [drafts, setDrafts] = useState<Record<string, RequestDraft>>({});
  const [outcomes, setOutcomes] = useState<Record<string, Outcome | undefined>>({});
  const [problems, setProblems] = useState<Record<string, BuildResult['problems']>>({});
  const [requestTab, setRequestTab] = useState<RequestTab>('params');
  const [responseTab, setResponseTab] = useState<ResponseTab>('body');
  const [sendingKey, setSendingKey] = useState<string | undefined>(undefined);
  const [copiedKey, setCopiedKey] = useState('');
  const [copyFailed, setCopyFailed] = useState(false);

  // A different API definition invalidates everything keyed by its operations:
  // drafts, responses and problems would otherwise carry over to a changed endpoint.
  const [seenModel, setSeenModel] = useState(model);
  if (seenModel !== model) {
    setSeenModel(model);
    setDrafts({});
    setOutcomes({});
    setProblems({});
    setSelectedSchema(undefined);
  }

  const abort = useRef<AbortController | undefined>(undefined);
  const copyTimer = useRef<number | undefined>(undefined);

  // A new document may not contain the selected operation any more.
  const operation = model.operations.find((o) => o.key === selectedKey) ?? model.operations[0];

  const draft = useMemo(
    () => (operation ? (drafts[operation.key] ?? initialDraft(operation)) : undefined),
    [drafts, operation],
  );

  const built = useMemo(
    () =>
      operation && draft
        ? buildSendRequest({ autoHeaders, autoQuery, baseUrl, draft, operation })
        : undefined,
    [autoHeaders, autoQuery, baseUrl, draft, operation],
  );

  // Mirror the latest valid request, once per change.
  const lastSignature = useRef<string | undefined>(undefined);
  useEffect(() => {
    if (!built?.request || !onRequestChange) return;
    const signature = requestSignature(built.request);
    if (signature === lastSignature.current) return;
    lastSignature.current = signature;
    onRequestChange(built.request);
  }, [built, onRequestChange]);

  // Stop anything in flight when the console goes away.
  useEffect(
    () => () => {
      abort.current?.abort();
      window.clearTimeout(copyTimer.current);
    },
    [],
  );

  const copy = useCallback((text: string, key: string) => {
    void writeClipboard(text).then((copied) => {
      window.clearTimeout(copyTimer.current);
      setCopyFailed(!copied);
      // "Copied" is only shown when the browser really took the text.
      setCopiedKey(copied ? key : '');
      if (copied) copyTimer.current = window.setTimeout(() => setCopiedKey(''), 1400);
    });
  }, []);

  const updateDraft = useCallback(
    (update: (current: RequestDraft) => RequestDraft) => {
      if (!operation) return;
      setDrafts((all) => ({
        ...all,
        [operation.key]: update(all[operation.key] ?? initialDraft(operation)),
      }));
      // Editing clears that operation's shown problems; they are recomputed on the next send.
      setProblems((all) => (all[operation.key] ? { ...all, [operation.key]: {} } : all));
    },
    [operation],
  );

  const cancel = useCallback(() => {
    abort.current?.abort();
  }, []);

  const send = useCallback(async () => {
    if (!operation || !draft || sendingKey) return;

    const result = buildSendRequest({ autoHeaders, autoQuery, baseUrl, draft, operation });
    setProblems((all) => ({ ...all, [operation.key]: result.problems }));

    if (!result.request) {
      // Take the user to the first tab that has something to fix.
      const keys = Object.keys(result.problems);
      if (keys.some((key) => key.startsWith('param:') || key === 'path')) setRequestTab('params');
      else if (keys.some((key) => key.startsWith('header:'))) setRequestTab('headers');
      else setRequestTab('body');
      return;
    }

    const controller = new AbortController();
    abort.current = controller;
    setSendingKey(operation.key);
    setResponseTab('body');
    const sent = {
      request: result.request,
      routeLabel,
      secretHeaders: result.secretHeaders,
      secretQuery: result.secretQuery,
    };

    try {
      const response = await transport(result.request, controller.signal);
      setOutcomes((all) => ({ ...all, [operation.key]: { result: response, sent } }));
    } catch (error) {
      if (error instanceof ConsoleTransportError && error.aborted) {
        // The user cancelled; there is nothing to report.
        return;
      }
      // Only our own, already-worded messages are shown. Anything else is not
      // passed through: an unknown error's text could carry URLs or tokens.
      const message =
        error instanceof ConsoleTransportError
          ? error.message
          : intl.formatMessage(messages.unexpected);
      setOutcomes((all) => ({ ...all, [operation.key]: { error: message, sent } }));
    } finally {
      setSendingKey(undefined);
    }
  }, [autoHeaders, autoQuery, baseUrl, draft, intl, operation, routeLabel, sendingKey, transport]);

  const onKeyDown = (event: KeyboardEvent<HTMLElement>) => {
    if ((event.metaKey || event.ctrlKey) && event.key === 'Enter') {
      event.preventDefault();
      void send();
    } else if (event.key === 'Escape' && sendingKey) {
      cancel();
    }
  };

  const lastStatus = useMemo(
    () =>
      Object.fromEntries(
        Object.entries(outcomes).map(([key, outcome]) => [key, outcome?.result?.status]),
      ),
    [outcomes],
  );

  const schema = model.schemas.find((s) => s.name === selectedSchema);
  const sending = operation !== undefined && sendingKey === operation.key;

  if (model.operations.length === 0) {
    return (
      <SecondaryText variant="body2">
        <FormattedMessage {...messages.noOperations} />
      </SecondaryText>
    );
  }

  return (
    <Grid container onKeyDown={onKeyDown} spacing={2}>
      <Grid size={{ lg: 4, xs: 12 }}>
        <OperationSidebar
          lastStatus={lastStatus}
          method={method}
          onMethod={setMethod}
          onSearch={setSearch}
          onSelectOperation={(key) => {
            setSelectedKey(key);
            setSelectedSchema(undefined);
            setResponseTab('body');
            setRequestTab('params');
          }}
          onSelectSchema={setSelectedSchema}
          operations={model.operations}
          schemas={model.schemas}
          search={search}
          selectedKey={operation?.key}
          selectedSchema={schema?.name}
        />
      </Grid>

      <Grid size={{ lg: 8, xs: 12 }} sx={{ minWidth: 0 }}>
        <Stack spacing={2}>
          {copyFailed && (
            <Alert onClose={() => setCopyFailed(false)} severity="warning">
              <FormattedMessage {...messages.copyFailed} />
            </Alert>
          )}
          {schema ? (
            <Card variant="outlined">
              <CardContent>
                <SchemaPage
                  onOpenOperation={(key) => {
                    setSelectedKey(key);
                    setSelectedSchema(undefined);
                  }}
                  operations={model.operations}
                  schema={schema}
                />
              </CardContent>
            </Card>
          ) : (
            operation &&
            draft &&
            built && (
              <>
                <RequestPanel
                  autoHeaders={autoHeaders}
                  autoQuery={autoQuery}
                  baseUrl={baseUrl.trim().replace(/\/+$/, '')}
                  copied={copiedKey === 'url'}
                  copyableUrl={displayUrl(
                    built.request ?? {
                      baseUrl: baseUrl.trim().replace(/\/+$/, ''),
                      headers: [],
                      method: operation.method,
                      path: built.preview.path,
                      query: built.preview.query,
                    },
                    { reveal: true },
                  )}
                  draft={draft}
                  onCancel={cancel}
                  onCopy={(text) => copy(text, 'url')}
                  onDraft={updateDraft}
                  onSend={() => void send()}
                  onTab={setRequestTab}
                  operation={operation}
                  preview={built.preview}
                  problems={problems[operation.key] ?? {}}
                  busy={sendingKey !== undefined && !sending}
                  sending={sending}
                  tab={requestTab}
                />
                <ResponsePanel
                  copiedKey={copiedKey}
                  onClear={() => setOutcomes((all) => ({ ...all, [operation.key]: undefined }))}
                  onCopy={copy}
                  onTab={setResponseTab}
                  operation={operation}
                  outcome={outcomes[operation.key]}
                  routeLabel={routeLabel}
                  sending={sending}
                  tab={responseTab}
                />
              </>
            )
          )}
        </Stack>
      </Grid>
    </Grid>
  );
}
