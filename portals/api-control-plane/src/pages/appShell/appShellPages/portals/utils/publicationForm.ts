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
import { isHttpUrl } from '../../apis/utils/basicInfoRules';

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
});

/**
 * Form state and the pre-fill chain for the alpha "Publish to Portal" editor —
 * the API Details fields this release covers
 * (`displayName`, `version`, `description`, the two endpoint URLs). Everything
 * else `PublicationDetailsCore` carries (tags, labels, agentVisibility, owners,
 * subscriptionPlanIds, docIds) belongs to a tab this alpha doesn't show, so a
 * save never sends it — a save sends every field currently shown, not every
 * field the schema could hold.
 */
export type DraftFormValues = {
  displayName: string;
  version: string;
  description: string;
  productionUrl: string;
  sandboxUrl: string;
};

export const emptyDraftFormValues: DraftFormValues = {
  displayName: '',
  version: '',
  description: '',
  productionUrl: '',
  sandboxUrl: '',
};

/**
 * The read chain, only for pre-filling the form: draft, then
 * publication, then the API's own — starting from the most recent edit,
 * published or not. No endpoint resolves this chain server-side, so it's
 * composed here from three independently-fetched tiers, each `undefined` when
 * that tier has nothing (a 404, not a real error — see `useApiPublicationDraft`).
 */
export const resolveDraftFormValues = (
  draft: PublicationDraftDetails | undefined,
  publication: Publication | undefined,
  api: RestApi | undefined,
): DraftFormValues => {
  if (draft) {
    return {
      displayName: draft.displayName ?? '',
      version: draft.version ?? '',
      description: draft.description ?? '',
      productionUrl: draft.endpoints?.productionUrl ?? '',
      sandboxUrl: draft.endpoints?.sandboxUrl ?? '',
    };
  }
  if (publication) {
    return {
      displayName: publication.displayName ?? '',
      version: publication.version ?? '',
      description: publication.description ?? '',
      productionUrl: publication.endpoints?.productionUrl ?? '',
      sandboxUrl: publication.endpoints?.sandboxUrl ?? '',
    };
  }
  if (api) {
    return {
      displayName: api.displayName ?? '',
      version: api.version ?? '',
      description: api.description ?? '',
      productionUrl: api.upstream?.main?.url ?? '',
      sandboxUrl: api.upstream?.sandbox?.url ?? '',
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
    // Sent only when at least one URL is set; omitted when both are empty.
    ...((productionUrl ?? sandboxUrl) !== undefined
      ? { endpoints: { productionUrl, sandboxUrl } }
      : {}),
  };
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
