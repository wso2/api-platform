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

import { useCallback } from 'react';
import { defineMessages } from 'react-intl';

import { useGraphQLApiIdAvailability } from '@/api/resources/graphqlApis';
import { versionLabel as toVersionSegment } from '@/utils/versionLabel';
import type { GeneralApiCreationFormState, GraphqlApiCreationFormState, GraphqlCreationWizardDraftState } from '../../types';
import type { CreateApiFormErrors } from '../../utils/serverFieldErrors';
import { GeneralCreateApiForm, type CreateApiFormProfile } from '../GeneralCreateApiForm';

export type GraphqlConfigureFormProps = {
  formId?: string;
  hideActions?: boolean;
  initialValues?: GraphqlCreationWizardDraftState;
  onSubmit: (values: GraphqlApiCreationFormState) => void;
  onBack: () => void;
  /** Why the last submission was rejected, when there was one. See `GeneralCreateApiForm`. */
  serverErrors?: CreateApiFormErrors;
  /**
   * Reports whether submitting is currently blocked by a taken identifier, for
   * a host that renders its own Create button (`hideActions`) and should
   * disable it exactly as this form's own button is disabled.
   */
  onSubmitBlockedChange?: (blocked: boolean) => void;
};

const messages = defineMessages({
  endpointErrorInvalid: {
    id: 'api.create.graphql.configureForm.endpoint.error.invalid',
    defaultMessage: 'Enter a full URL, for example https://api.example.com/graphql.',
  },
  endpointErrorRequired: {
    id: 'api.create.graphql.configureForm.endpoint.error.required',
    defaultMessage: 'Enter the GraphQL endpoint URL.',
  },
  endpointLabel: {
    id: 'api.create.graphql.configureForm.endpoint.label',
    defaultMessage: 'Query and Mutation URL',
  },
  endpointSection: {
    id: 'apiControlPlane.pages.test.console.GatewaySection.endpoint',
    defaultMessage: 'Endpoint',
    description: 'Label above the URL that requests from this console are sent to. Shown in capitals by the layout, so translate it as ordinary words.',
  },
});

/**
 * The routing context this platform gives a GraphQL API: all operations are
 * served from one path (`/{api-handler}/v{version}/graphql`), unlike REST's
 * project-prefixed base path — a GraphQL API's project scope comes from
 * `projectId`, not the URL. The trailing `graphql` segment matches the
 * platform-api spec's own documented convention for a GraphQL endpoint's
 * context, the same way the gateway reserves `/mcp` for an MCP proxy.
 */
const toContext = (apiHandle: string, version: string): string => {
  const segments = [
    apiHandle.trim(),
    version.trim() === '' ? undefined : toVersionSegment(version.trim()),
    'graphql',
  ].filter((segment): segment is string => Boolean(segment));

  return `/${segments.join('/')}`;
};

/**
 * GraphQL's profile of the shared configure form: its own context rule and
 * endpoint wording, an optional context (the server derives one), and the live
 * identifier check that blocks Create on a taken handle.
 */
const GRAPHQL_FORM_PROFILE: CreateApiFormProfile = {
  apiKind: 'graphql',
  contextRequired: false,
  deriveContext: (_projectHandler, apiHandle, version) => toContext(apiHandle, version),
  endpointErrorInvalid: messages.endpointErrorInvalid,
  endpointErrorRequired: messages.endpointErrorRequired,
  endpointLabel: messages.endpointLabel,
  endpointSection: messages.endpointSection,
  useIdentifierAvailability: useGraphQLApiIdAvailability,
};

/** The GraphQL draft as the shared form's state: its endpoint is REST's `upstream.main.url`. */
const toFormDraft = (draft: GraphqlCreationWizardDraftState): Partial<GeneralApiCreationFormState> => ({
  ...(draft.id === undefined ? {} : { id: draft.id }),
  ...(draft.displayName === undefined ? {} : { displayName: draft.displayName }),
  ...(draft.description === undefined ? {} : { description: draft.description }),
  ...(draft.version === undefined ? {} : { version: draft.version }),
  ...(draft.context === undefined ? {} : { context: draft.context }),
  upstream: { main: { url: draft.endpointUrl ?? '' } },
});

/**
 * The GraphQL wizard's "Configure and create" step: REST's own configure form
 * (`GeneralCreateApiForm`) under GraphQL's profile, so both API types share one
 * set of fields, rules and layout. The schema the source step resolved rides
 * along untouched and is merged back into what this step submits.
 */
export const GraphqlConfigureForm = (props: GraphqlConfigureFormProps) => {
  const draft = props.initialValues ?? {};
  const { onSubmit } = props;

  const submit = useCallback(
    (values: GeneralApiCreationFormState) =>
      onSubmit({
        ...(draft.sdl === undefined ? {} : { sdl: draft.sdl }),
        ...(draft.sdlUrl === undefined ? {} : { sdlUrl: draft.sdlUrl }),
        ...(draft.sdlFile === undefined ? {} : { sdlFile: draft.sdlFile }),
        context: values.context,
        description: values.description,
        displayName: values.displayName,
        endpointUrl: values.upstream.main.url,
        id: values.id,
        schemaSource: draft.schemaSource ?? 'introspection',
        version: values.version,
      }),
    [draft.schemaSource, draft.sdl, draft.sdlFile, draft.sdlUrl, onSubmit],
  );

  return (
    <GeneralCreateApiForm
      formId={props.formId}
      hideActions={props.hideActions}
      // A draft that already names an identifier or context (a returned-to
      // submission) keeps it rather than re-deriving it from the name.
      initialBasePathEdited={(draft.context ?? '').trim() !== ''}
      initialIdentifierEdited={(draft.id ?? '').trim() !== ''}
      initialValues={toFormDraft(draft)}
      onBack={props.onBack}
      onSubmit={submit}
      onSubmitBlockedChange={props.onSubmitBlockedChange}
      profile={GRAPHQL_FORM_PROFILE}
      serverErrors={props.serverErrors}
    />
  );
};
