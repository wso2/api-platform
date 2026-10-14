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

import { FileCode2, Pencil } from '@wso2/oxygen-ui-icons-react';
import { useEffect, useState } from 'react';
import { defineMessages, FormattedMessage } from 'react-intl';

import type { GraphqlCreationWizardDraftState } from '../../types';
import { DefineApproachLayout, type DefineApproach } from '../DefineApproachLayout';
import { GraphqlIntrospectionForm } from './GraphqlIntrospectionForm';
import { GraphqlSchemaExplorer } from './GraphqlSchemaExplorer';
import { GraphqlUrlUploadForm } from './GraphqlUrlUploadForm';
import type { GraphqlResolutionFailure, GraphqlResolvedSchema } from './graphqlSourceTypes';

type ApproachKey = 'schema' | 'scratch';

const messages = defineMessages({
  schemaDescription: {
    id: 'api.create.graphql.definePanel.schema.description',
    defaultMessage: 'Import from a URL or upload a schema file.',
  },
  schemaTitle: {
    id: 'api.create.graphql.definePanel.schema.title',
    defaultMessage: 'Start with a schema',
  },
  scratchDescription: {
    id: 'api.create.graphql.definePanel.scratch.description',
    defaultMessage: 'Start blank, or seed the schema from a backend endpoint.',
  },
  scratchTitle: {
    id: 'api.create.graphql.definePanel.scratch.title',
    defaultMessage: 'Start from scratch',
  },
});

/**
 * GraphQL's approaches, in the same order and with the same icons as REST's
 * (`DefineApiPanel`): Start from scratch, then the schema import that stands in
 * for REST's contract import.
 */
const APPROACHES: DefineApproach<ApproachKey>[] = [
  {
    description: <FormattedMessage {...messages.scratchDescription} />,
    icon: <Pencil size={20} />,
    key: 'scratch',
    title: <FormattedMessage {...messages.scratchTitle} />,
  },
  {
    description: <FormattedMessage {...messages.schemaDescription} />,
    icon: <FileCode2 size={20} />,
    key: 'schema',
    title: <FormattedMessage {...messages.schemaTitle} />,
  },
];

export type GraphqlDefinePanelProps = {
  /** Keeps the wizard footer supplied with the definition currently on screen. */
  onDraftChange: (data: GraphqlCreationWizardDraftState | null) => void;
};

/**
 * A starting guess at the API's name. Unlike REST's `extractApiDetails`,
 * which reads a real `info.title` out of the OpenAPI document, GraphQL SDL
 * and introspection carry no name field at all — so this falls back to the
 * imported file's name, or the endpoint/SDL URL's hostname, humanized. Always
 * just a suggestion: the configure step's own `identifierEdited` guard lets
 * the user override the name (and, in turn, the identifier it derives) by
 * hand, exactly as it already does for a REST import.
 */
const deriveDisplayName = (resolved: GraphqlResolvedSchema): string | undefined => {
  const humanize = (raw: string) =>
    raw
      .replace(/\.(graphql|gql|json)$/i, '')
      .replace(/[-_]+/g, ' ')
      .trim()
      .replace(/\b\w/g, (letter) => letter.toUpperCase());

  if (resolved.sdlFile) return humanize(resolved.sdlFile.name) || undefined;

  const url = resolved.endpointUrl ?? resolved.sdlUrl;
  if (!url) return undefined;
  try {
    const host = new URL(url).hostname.split('.')[0];
    return host ? humanize(host) : undefined;
  } catch {
    return undefined;
  }
};

/**
 * The GraphQL wizard's "how do you want to define this API?" step, built on
 * the same `DefineApproachLayout` as REST's `DefineApiPanel`.
 *
 * Both approaches share one schema explorer as the preview: importing a schema
 * (URL/file) or introspecting a backend endpoint funnel through the same
 * dry-run validation call and land in the same right-hand pane. Back and Next
 * belong to the wizard's shared footer, not this panel.
 */
export const GraphqlDefinePanel = ({ onDraftChange }: GraphqlDefinePanelProps) => {
  const [approach, setApproach] = useState<ApproachKey>('schema');
  const [resolved, setResolved] = useState<GraphqlResolvedSchema | null>(null);
  const [failure, setFailure] = useState<GraphqlResolutionFailure | null>(null);

  const handleApproachChange = (next: ApproachKey) => {
    if (next === approach) return;
    setApproach(next);
    setResolved(null);
    setFailure(null);
  };

  useEffect(() => {
    // Fields absent from `resolved` (e.g. `endpointUrl` for a URL/file source)
    // are omitted here rather than sent as an explicit `undefined` — the
    // configure form fills its defaults in via `{...DEFAULT, ...draft}`, and
    // an own property set to `undefined` would clobber that default instead
    // of falling through to it.
    const displayName = resolved === null ? undefined : deriveDisplayName(resolved);

    onDraftChange(
      resolved === null
        ? null
        : {
            schemaSource: resolved.schemaSource,
            ...(resolved.sdl === undefined ? {} : { sdl: resolved.sdl }),
            ...(resolved.sdlUrl === undefined ? {} : { sdlUrl: resolved.sdlUrl }),
            ...(resolved.sdlFile === undefined ? {} : { sdlFile: resolved.sdlFile }),
            ...(resolved.endpointUrl === undefined ? {} : { endpointUrl: resolved.endpointUrl }),
            ...(displayName === undefined ? {} : { displayName }),
          },
    );
    return () => onDraftChange(null);
  }, [onDraftChange, resolved]);

  return (
    <DefineApproachLayout
      approaches={APPROACHES}
      form={
        approach === 'schema' ? (
          <GraphqlUrlUploadForm onResolved={setResolved} onValidationFailed={setFailure} />
        ) : (
          <GraphqlIntrospectionForm onResolved={setResolved} />
        )
      }
      onChange={handleApproachChange}
      preview={<GraphqlSchemaExplorer error={failure} sdl={resolved?.sdl} />}
      value={approach}
    />
  );
};
