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

import { defineMessages, type MessageDescriptor } from 'react-intl';

import type { Publication, PublicationDraftDetails, PublicationDraftDetailsInput } from '@/api/resources/apiPublications';
import type { RestApi } from '@/api/resources/restApis';
import type { GraphQLApiGateway } from '@/api/resources/graphqlApis/apiGateways';
import type { RestApiGateway } from '@/api/resources/restApis/apiGateways/apiGateways.endpoints';
import { isHttpUrl } from '../../apis/utils/basicInfoRules';
import { buildInvokeUrl, gatewayEndpoint } from '../../gateways/utils/gatewayDisplay';

const messages = defineMessages({
  displayNameRequired: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.utils.publicationForm.displayNameRequired',
    defaultMessage: 'Enter a name.',
  },
  versionRequired: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.utils.publicationForm.versionRequired',
    defaultMessage: 'Enter a version.',
  },
  urlInvalid: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.utils.publicationForm.urlInvalid',
    defaultMessage: 'Enter a full URL, for example https://api.example.com.',
  },
  thumbnailType: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.utils.publicationForm.thumbnailType',
    defaultMessage: 'Only JPEG and PNG images are supported.',
  },
  thumbnailSize: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.utils.publicationForm.thumbnailSize',
    defaultMessage: 'The image must be 2 MB or smaller.',
  },
});

export type AgentVisibility = NonNullable<PublicationDraftDetailsInput['agentVisibility']>;

/** What the server stores for a listing that never set it; shown until the user chooses. */
const DEFAULT_AGENT_VISIBILITY: AgentVisibility = 'VISIBLE';

/** Form state for the publish editor: the fields its tabs show. A save sends every field shown, not every field the schema could hold. */
export type DraftFormValues = {
  displayName: string;
  version: string;
  description: string;
  productionUrl: string;
  sandboxUrl: string;
  agentVisibility: AgentVisibility;
  subscriptionPlanIds: string[];
  /** Handles of the API's documents published with it. */
  docIds: string[];
};

/** A URL the Production URL can be picked from, and the gateway that serves it. */
export type GatewayUrlOption = { gatewayName: string; url: string };

/**
 * The URLs the API is served at: one per gateway it is deployed to, in the
 * order the API lists them. A gateway without an address has no URL to offer.
 */
export const buildProductionUrlOptions = (
  gateways: readonly (RestApiGateway | GraphQLApiGateway)[],
  context: string | undefined,
): GatewayUrlOption[] =>
  gateways
    .filter((gateway) => gateway.isDeployed)
    .map((gateway) => ({ gatewayName: gateway.displayName, url: buildInvokeUrl(gatewayEndpoint(gateway), context) }))
    .filter((option) => option.url !== '');

/** A plan or document the listing still selects whose source has since been deleted; only its stored name is left. */
export type DeletedSelection = { id: string; displayName: string };

type SelectionList = Publication['subscriptionPlans'];
type SelectionSource = Pick<
  Publication | PublicationDraftDetails,
  'docIds' | 'documents' | 'subscriptionPlanIds' | 'subscriptionPlans'
>;
/** A saved listing, draft or live: the fields the form is filled from. */
type ListingSource = SelectionSource &
  Pick<Publication | PublicationDraftDetails, 'agentVisibility' | 'description' | 'displayName' | 'endpoints' | 'version'>;

/** The selected items that still exist; a deleted one cannot be saved or published, so it never enters the form. */
const existingIds = (selections: SelectionList, ids: string[] | undefined): string[] => {
  if (!selections) return ids ?? [];
  return selections.filter((item) => item.sourceStatus !== 'DELETED').map((item) => item.id);
};

const deletedOf = (selections: SelectionList): DeletedSelection[] =>
  (selections ?? []).filter((item) => item.sourceStatus === 'DELETED').map(({ id, displayName }) => ({ id, displayName }));

const editedIdsOf = (selections: SelectionList): string[] =>
  (selections ?? []).filter((item) => item.sourceStatus === 'UPDATED').map((item) => item.id);

/** The names stored with the selection, by id, as of the last save or publish. */
const storedNamesOf = (selections: SelectionList): Record<string, string> =>
  Object.fromEntries((selections ?? []).map((item) => [item.id, item.displayName]));

/** The selected plans whose subscription plan was deleted, for the "Deleted" cards. */
export const deletedPlansOf = (source: SelectionSource | undefined): DeletedSelection[] =>
  deletedOf(source?.subscriptionPlans);

/** The ids of selected plans changed in Settings after the selection was last saved or published. */
export const editedPlanIdsOf = (source: SelectionSource | undefined): string[] =>
  editedIdsOf(source?.subscriptionPlans);

export const planNamesOf = (source: SelectionSource | undefined): Record<string, string> =>
  storedNamesOf(source?.subscriptionPlans);

/** The selected documents that were deleted, for the "Deleted" rows. */
export const deletedDocumentsOf = (source: SelectionSource | undefined): DeletedSelection[] =>
  deletedOf(source?.documents);

/** The ids of selected documents changed after the selection was last saved or published. */
export const editedDocIdsOf = (source: SelectionSource | undefined): string[] => editedIdsOf(source?.documents);

export const documentNamesOf = (source: SelectionSource | undefined): Record<string, string> =>
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

/** The API fields a draft starts from — REST and GraphQL APIs both carry them. */
export type PublishableApiDetails = Pick<RestApi, 'description' | 'displayName' | 'version'>;

const valuesOfListing = (listing: ListingSource): DraftFormValues => ({
  displayName: listing.displayName ?? '',
  version: listing.version ?? '',
  description: listing.description ?? '',
  productionUrl: listing.endpoints?.productionUrl ?? '',
  sandboxUrl: listing.endpoints?.sandboxUrl ?? '',
  agentVisibility: listing.agentVisibility ?? DEFAULT_AGENT_VISIBILITY,
  subscriptionPlanIds: existingIds(listing.subscriptionPlans, listing.subscriptionPlanIds),
  docIds: existingIds(listing.documents, listing.docIds),
});

/**
 * The read chain, only for pre-filling the form: draft, then
 * publication, then the API's own — starting from the most recent edit,
 * published or not. No endpoint resolves this chain server-side, so it's
 * composed here from three independently-fetched tiers, each `undefined` when
 * that tier has nothing (a 404, not a real error — see `useApiPublicationDraft`).
 * The API's own tier defaults the Production URL to `defaultProductionUrl`, a
 * gateway URL, and leaves it empty without one; the Sandbox URL starts empty.
 * The API's backend URLs are not exposed.
 */
export const resolveDraftFormValues = (
  draft: PublicationDraftDetails | undefined,
  publication: Publication | undefined,
  api: PublishableApiDetails | undefined,
  defaultProductionUrl?: string,
): DraftFormValues => {
  if (draft) return valuesOfListing(draft);
  if (publication) return valuesOfListing(publication);
  if (api) {
    return {
      displayName: api.displayName ?? '',
      version: api.version ?? '',
      description: api.description ?? '',
      productionUrl: defaultProductionUrl ?? '',
      sandboxUrl: '',
      agentVisibility: DEFAULT_AGENT_VISIBILITY,
      subscriptionPlanIds: [],
      docIds: [],
    };
  }
  return emptyDraftFormValues;
};

/**
 * The PUT body for `.../draft` — every field the alpha form shows, saved
 * together rather than field by field. Empty strings are
 * sent as `undefined` rather than `''`, so a field the user never touched
 * reads back as unset instead of an empty value.
 */
export const draftFormValuesToInput = (values: DraftFormValues): PublicationDraftDetailsInput => {
  const trimmedOrUndefined = (value: string): string | undefined => {
    const trimmed = value.trim();
    return trimmed === '' ? undefined : trimmed;
  };

  const productionUrl = trimmedOrUndefined(values.productionUrl);
  const sandboxUrl = trimmedOrUndefined(values.sandboxUrl);

  return {
    displayName: values.displayName.trim(),
    version: values.version.trim(),
    description: trimmedOrUndefined(values.description),
    agentVisibility: values.agentVisibility,
    // Sent only when at least one URL is set; omitted when both are empty.
    ...((productionUrl ?? sandboxUrl) !== undefined
      ? { endpoints: { productionUrl, sandboxUrl } }
      : {}),
    // Always sent, even as `[]`: the draft PUT replaces both selections, so leaving one out
    // would clear it.
    subscriptionPlanIds: values.subscriptionPlanIds,
    docIds: values.docIds,
  };
};

/** A selection is a set: the server ignores order, and an item toggled off and on moves to the end. */
export const sameSelection = (a: string[], b: string[]) => {
  if (a === b) return true;
  const members = new Set(a);
  return members.size === new Set(b).size && b.every((id) => members.has(id));
};

/** Whether two sets of form values hold the same content, comparing each selection as a set. */
export const sameDraftFormValues = (a: DraftFormValues, b: DraftFormValues): boolean =>
  (Object.keys(a) as (keyof DraftFormValues)[]).every((field) => {
    const left = a[field];
    const right = b[field];
    return Array.isArray(left) && Array.isArray(right) ? sameSelection(left, right) : left === right;
  });

/** The selection with `id` added, or removed when it is already there. */
export const toggleSelection = (ids: readonly string[], id: string): string[] =>
  ids.includes(id) ? ids.filter((selected) => selected !== id) : [...ids, id];

const THUMBNAIL_TYPES: readonly string[] = ['image/jpeg', 'image/png'];
/** The server's default cap on a draft thumbnail (`publication_thumbnail_max_bytes`), checked before uploading. */
const THUMBNAIL_MAX_BYTES = 2 * 1024 * 1024;

/** Why a picked image can't be the thumbnail, or `undefined`. The server re-checks the type. */
export const validateThumbnailFile = (file: Pick<File, 'size' | 'type'>): MessageDescriptor | undefined => {
  if (!THUMBNAIL_TYPES.includes(file.type)) return messages.thumbnailType;
  if (file.size > THUMBNAIL_MAX_BYTES) return messages.thumbnailSize;
  return undefined;
};

export type DraftFormField = 'displayName' | 'version' | 'productionUrl' | 'sandboxUrl';
export type FormFieldErrors = Partial<Record<DraftFormField, MessageDescriptor>>;

/**
 * `displayName`/`version` are the schema's own required fields
 * (`PublicationDraftDetailsInput`); the two URLs are optional but, when given,
 * have to be full URLs — the same rule the API creation form applies to its
 * own target URL. Returns `MessageDescriptor`s, not strings, so the caller
 * renders them with `<FormattedMessage>` — this module has no `IntlProvider`.
 */
export const validateDraftFormValues = (values: DraftFormValues): FormFieldErrors => {
  const errors: FormFieldErrors = {};

  if (values.displayName.trim() === '') errors.displayName = messages.displayNameRequired;
  if (values.version.trim() === '') errors.version = messages.versionRequired;

  const productionUrl = values.productionUrl.trim();
  if (productionUrl !== '' && !isHttpUrl(productionUrl)) {
    errors.productionUrl = messages.urlInvalid;
  }

  const sandboxUrl = values.sandboxUrl.trim();
  if (sandboxUrl !== '' && !isHttpUrl(sandboxUrl)) {
    errors.sandboxUrl = messages.urlInvalid;
  }

  return errors;
};
