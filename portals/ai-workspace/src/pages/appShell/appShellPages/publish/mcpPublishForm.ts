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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import { defineMessages, type MessageDescriptor } from 'react-intl';

import type { Gateway } from '../../../../apis/gatewayTypes';
import { buildMcpInvokeUrl } from '../../../../utils/mcpInvokeUrl';
import type {
  ApiPortalAgentVisibility,
  MCPServer,
  PublicationDetailsCore,
  PublicationDraftDetailsInput,
  PublicationSelection,
} from '../../../../utils/types';

/**
 * Form state and the pre-fill chain for the publish page: name, version,
 * description, the two endpoint URLs, agent visibility, and the selected plans
 * and documents. Tags, labels and owners are not shown on this page, so a save
 * carries over whatever the stored draft (or live listing) already holds rather
 * than clearing it.
 */

/** What the server stores for a listing that never set it; shown until the user chooses. */
const DEFAULT_AGENT_VISIBILITY: ApiPortalAgentVisibility = 'VISIBLE';

export type DraftFormValues = {
  displayName: string;
  version: string;
  description: string;
  productionUrl: string;
  sandboxUrl: string;
  agentVisibility: ApiPortalAgentVisibility;
  /** Handles of the subscription plans the listing offers. */
  subscriptionPlanIds: string[];
  /** Handles of the proxy's documents published with it. */
  docIds: string[];
};

export type DraftFormField = 'displayName' | 'version' | 'productionUrl' | 'sandboxUrl';

type FormFieldErrors = Partial<Record<DraftFormField, MessageDescriptor>>;

export const publishFormMessages = defineMessages({
  nameRequired: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.form.nameRequired',
    defaultMessage: 'Enter a name.',
  },
  versionRequired: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.form.versionRequired',
    defaultMessage: 'Enter a version.',
  },
  urlInvalid: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.form.urlInvalid',
    defaultMessage: 'Enter a full URL, for example https://mcp.example.com.',
  },
  thumbnailType: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.form.thumbnailType',
    defaultMessage: 'Only JPEG and PNG images are supported.',
  },
  thumbnailSize: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.form.thumbnailSize',
    defaultMessage: 'The image must be 2 MB or smaller.',
  },
});

/** A URL the Production URL can be picked from, and the gateway that serves it. */
export type GatewayUrlOption = { gatewayName: string; url: string };

/** A plan or document the listing still selects whose source has since been deleted; only its stored name is left. */
export type DeletedSelection = { id: string; displayName: string };

/** What a draft or the live listing holds, as far as the form is concerned. */
type StoredPublication = Partial<PublicationDetailsCore> &
  Pick<PublicationDraftDetailsInput, 'subscriptionPlanIds' | 'docIds'> & {
    subscriptionPlans?: PublicationSelection[];
    documents?: PublicationSelection[];
  };

/** The selected items that still exist; a deleted one cannot be saved or published, so it never enters the form. */
const existingIds = (selections: PublicationSelection[] | undefined, ids: string[] | undefined): string[] => {
  if (!selections) return ids ?? [];
  return selections.filter((item) => !item.deleted).map((item) => item.id);
};

const deletedOf = (selections: PublicationSelection[] | undefined): DeletedSelection[] =>
  (selections ?? []).filter((item) => item.deleted).map(({ id, displayName }) => ({ id, displayName }));

const editedIdsOf = (selections: PublicationSelection[] | undefined): string[] =>
  (selections ?? []).filter((item) => item.edited && !item.deleted).map((item) => item.id);

/** The names stored with the selection, by id, as of the last save or publish. */
const storedNamesOf = (selections: PublicationSelection[] | undefined): Record<string, string> =>
  Object.fromEntries((selections ?? []).map((item) => [item.id, item.displayName]));

export const deletedPlansOf = (source: StoredPublication | undefined): DeletedSelection[] =>
  deletedOf(source?.subscriptionPlans);

export const editedPlanIdsOf = (source: StoredPublication | undefined): string[] =>
  editedIdsOf(source?.subscriptionPlans);

export const planNamesOf = (source: StoredPublication | undefined): Record<string, string> =>
  storedNamesOf(source?.subscriptionPlans);

export const deletedDocumentsOf = (source: StoredPublication | undefined): DeletedSelection[] =>
  deletedOf(source?.documents);

export const editedDocIdsOf = (source: StoredPublication | undefined): string[] => editedIdsOf(source?.documents);

export const documentNamesOf = (source: StoredPublication | undefined): Record<string, string> =>
  storedNamesOf(source?.documents);

export const emptyDraftFormValues: DraftFormValues = {
  displayName: '',
  version: '',
  description: '',
  productionUrl: '',
  sandboxUrl: '',
  agentVisibility: DEFAULT_AGENT_VISIBILITY,
  subscriptionPlanIds: [],
  docIds: [],
};

/** The URLs the proxy is served at: one per gateway it is deployed to, newest deployment first. */
export const buildProductionUrlOptions = (
  gateways: readonly Gateway[],
  context: string | undefined
): GatewayUrlOption[] =>
  gateways
    .map((gateway) => ({
      gatewayName: gateway.displayName || gateway.name,
      url: buildMcpInvokeUrl(gateway, context),
    }))
    .filter((option) => option.url !== '');

const fromStored = (stored: StoredPublication): DraftFormValues => ({
  displayName: stored.displayName ?? '',
  version: stored.version ?? '',
  description: stored.description ?? '',
  productionUrl: stored.endpoints?.productionUrl ?? '',
  sandboxUrl: stored.endpoints?.sandboxUrl ?? '',
  agentVisibility: stored.agentVisibility ?? DEFAULT_AGENT_VISIBILITY,
  subscriptionPlanIds: existingIds(stored.subscriptionPlans, stored.subscriptionPlanIds),
  docIds: existingIds(stored.documents, stored.docIds),
});

/**
 * Pre-fills the form from the draft, else the live listing, else the proxy itself, whose
 * Production URL defaults to `defaultProductionUrl`.
 */
export const resolveDraftFormValues = (
  draft: StoredPublication | undefined,
  publication: StoredPublication | undefined,
  server: Pick<MCPServer, 'displayName' | 'version' | 'description'> | undefined,
  defaultProductionUrl?: string
): DraftFormValues => {
  if (draft) return fromStored(draft);
  if (publication) return fromStored(publication);
  if (server) {
    return {
      ...emptyDraftFormValues,
      displayName: server.displayName ?? '',
      version: server.version ?? '',
      description: server.description ?? '',
      productionUrl: defaultProductionUrl ?? '',
      subscriptionPlanIds: [],
      docIds: [],
    };
  }
  return emptyDraftFormValues;
};

/** A selection is a set: the server ignores order, and an item toggled off and on moves to the end. */
export const sameSelection = (a: string[], b: string[]): boolean => {
  if (a === b) return true;
  const members = new Set(a);
  return members.size === new Set(b).size && b.every((id) => members.has(id));
};

const trimmedOrUndefined = (value: string): string | undefined => {
  const trimmed = value.trim();
  return trimmed === '' ? undefined : trimmed;
};

/**
 * The PUT body for `.../draft`. A save replaces the whole draft, so fields this page doesn't show
 * are carried over from `stored`. Empty strings are sent as `undefined`; the selections are always
 * sent, even as `[]`, since leaving one out would clear it.
 */
export const draftFormValuesToInput = (
  values: DraftFormValues,
  stored?: StoredPublication
): PublicationDraftDetailsInput => {
  const productionUrl = trimmedOrUndefined(values.productionUrl);
  const sandboxUrl = trimmedOrUndefined(values.sandboxUrl);
  return {
    displayName: values.displayName.trim(),
    version: values.version.trim(),
    description: trimmedOrUndefined(values.description),
    agentVisibility: values.agentVisibility,
    ...((productionUrl ?? sandboxUrl) !== undefined ? { endpoints: { productionUrl, sandboxUrl } } : {}),
    tags: stored?.tags,
    labels: stored?.labels,
    owners: stored?.owners,
    subscriptionPlanIds: values.subscriptionPlanIds,
    docIds: values.docIds,
  };
};

const isHttpUrl = (value: string): boolean => {
  try {
    const { protocol } = new URL(value);
    return protocol === 'http:' || protocol === 'https:';
  } catch {
    return false;
  }
};

/** Name and version are required; the URLs are optional but must be full URLs. */
export const validateDraftFormValues = (values: DraftFormValues): FormFieldErrors => {
  const errors: FormFieldErrors = {};
  if (values.displayName.trim() === '') errors.displayName = publishFormMessages.nameRequired;
  if (values.version.trim() === '') errors.version = publishFormMessages.versionRequired;

  const urlMessage = publishFormMessages.urlInvalid;
  const productionUrl = values.productionUrl.trim();
  if (productionUrl !== '' && !isHttpUrl(productionUrl)) errors.productionUrl = urlMessage;
  const sandboxUrl = values.sandboxUrl.trim();
  if (sandboxUrl !== '' && !isHttpUrl(sandboxUrl)) errors.sandboxUrl = urlMessage;
  return errors;
};

const THUMBNAIL_TYPES: readonly string[] = ['image/jpeg', 'image/png'];
/** The server's default cap on a draft thumbnail (`publication_thumbnail_max_bytes`), checked before uploading. */
const THUMBNAIL_MAX_BYTES = 2 * 1024 * 1024;

/** Why a picked image can't be the thumbnail, or `undefined`. The server re-checks the type from the bytes. */
export const validateThumbnailFile = (file: Pick<File, 'size' | 'type'>): MessageDescriptor | undefined => {
  if (!THUMBNAIL_TYPES.includes(file.type)) return publishFormMessages.thumbnailType;
  if (file.size > THUMBNAIL_MAX_BYTES) return publishFormMessages.thumbnailSize;
  return undefined;
};

/** What the listing advertises: tools, resources and prompts, and anything else the server keeps with them. */
export type McpDefinition = Record<string, unknown>;

/** The definition as JSON text, with the three lists always present and first; other fields follow untouched. */
export const definitionToText = (definition: McpDefinition | undefined): string => {
  const { tools = [], resources = [], prompts = [], ...rest } = definition ?? {};
  return JSON.stringify({ tools, resources, prompts, ...rest }, null, 2);
};

/** The text is only ever written by `definitionToText` or the Capabilities tab, so it is always a JSON object. */
export const parseDefinitionText = (text: string): McpDefinition =>
  text.trim() === '' ? {} : (JSON.parse(text) as McpDefinition);

/**
 * The monogram shown for a name: its first two letters, not word initials —
 * "Travel MCP" reads as `TR`, which is easier to match back to the title.
 */
export const nameInitials = (name: string): string =>
  name.replace(/[^\p{L}\p{N}]/gu, '').slice(0, 2).toUpperCase();
