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

import { useEffect, useRef } from 'react';
import { defineMessages, useIntl } from 'react-intl';

import { isApiError } from '@/api/core/errors';
import { useAllApiDocuments } from '@/api/resources/apiDocuments';
import {
  usePublishApiToApiPortal,
  useSaveApiPublicationDraft,
  useSaveApiPublicationDraftDefinition,
  useSaveApiPublicationDraftThumbnail,
  type DraftDefinitionBody,
} from '@/api/resources/apiPublications';
import { useNotifications } from '@/components/Notifications';
import { parseSpecText } from '../../apis/create/utils/specText';
import type { PublishTarget } from '../publishTarget';
import { usePublishPageData } from '../usePublishPageData';
import {
  draftFormValuesToInput,
  validateDraftFormValues,
  validateThumbnailFile,
} from '../utils/publicationForm';
import type { StoredDefinition } from '../utils/storedDefinition';

const messages = defineMessages({
  published: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.PublishWithDefaults.published',
    defaultMessage: 'Published {apiName} to {portalName}.',
    description:
      'Shown once a one-click Publish from a portal card succeeds. {apiName} and {portalName} are user-supplied.',
  },
  needsReview: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.PublishWithDefaults.needsReview',
    defaultMessage: 'Nothing was published to {portalName}. Check the details here, then publish.',
    description:
      'Shown when a one-click Publish from a portal card cannot publish as it is and opens the publish page instead. {portalName} is user-supplied.',
  },
});

/**
 * The definition as the draft endpoint takes it; undefined when there is none or it does not parse.
 * It is the API's own, which the server validated on upload with the same check publish runs.
 */
const definitionBodyOf = (
  target: PublishTarget,
  definition: StoredDefinition | undefined,
): DraftDefinitionBody | undefined => {
  if (!definition || definition.text.trim() === '') return undefined;
  if (target.definitionKind === 'graphql') return { sdl: definition.text };
  if (definition.format === 'graphql') return undefined;
  const parsed = parseSpecText(definition.text, definition.format);
  return parsed.status === 'parsed' ? parsed.spec : undefined;
};

type PublishWithDefaultsProps = {
  /**
   * The page now shows another API, in this organization or another. Before anything is written the
   * publish is dropped without a word, as when the page is left, rather than writing or opening a page
   * the user has moved away from; once writing has begun it finishes, in the organization of the
   * `ApiScopeProvider` above it.
   */
  apiChanged: boolean;
  apiPortalId: string;
  /** Called once this publish is over: it went through, failed, or was handed to the publish page. */
  onDone: () => void;
  /** Reads the listing again once a step has failed, so a draft saved before it shows on the card. */
  onFailed: () => Promise<unknown>;
  /** Called instead of publishing when the publish page must decide; see below. */
  onOpenPage: () => void;
  portalName: string;
  target: PublishTarget;
};

/**
 * The portal card's one-click Publish, run without leaving the card: the API's own details,
 * Production URL, thumbnail and specification, every document, no plan and no landing page.
 * Mounted for one publish and renders nothing. The page is opened instead when something
 * is saved for this portal after all, a read fails, the document list is cut short, the API
 * has no specification or one that does not parse, or its details or thumbnail are sure to be
 * refused.
 */
export function PublishWithDefaults({
  apiChanged,
  apiPortalId,
  onDone,
  onFailed,
  onOpenPage,
  portalName,
  target,
}: PublishWithDefaultsProps) {
  const intl = useIntl();
  const { notify } = useNotifications();
  const data = usePublishPageData(apiPortalId, target, false);
  const documentsQuery = useAllApiDocuments(target.apiType, target.apiHandle);
  const saveDraft = useSaveApiPublicationDraft();
  const saveDefinition = useSaveApiPublicationDraftDefinition();
  const saveThumbnail = useSaveApiPublicationDraftThumbnail();
  const publish = usePublishApiToApiPortal();
  const started = useRef(false);

  useEffect(() => {
    if (started.current) return;
    if (apiChanged) {
      started.current = true;
      onDone();
      return;
    }
    if (data.isLoading || documentsQuery.isPending || documentsQuery.isFetching) return;
    started.current = true;

    const openPage = () => {
      notify(intl.formatMessage(messages.needsReview, { portalName }), 'info');
      onDone();
      onOpenPage();
    };
    const documents = documentsQuery.data;
    const { seed } = data;
    // A failed refetch keeps the cached list beside its error, so the error is checked, not just the list.
    if (
      !seed ||
      !documents ||
      documentsQuery.error ||
      documents.list.length < documents.pagination.total ||
      data.error ||
      data.productionUrlsFailed ||
      data.apiThumbnail.error ||
      data.draft ||
      data.publication
    ) {
      openPage();
      return;
    }
    const values = {
      ...seed.values,
      docIds: documents.list.map((doc) => doc.id),
      subscriptionPlanIds: [],
    };
    const definition = definitionBodyOf(target, seed.definition);
    // Checked before anything is written, so none of these refusals leaves a half-saved draft behind.
    if (
      Object.keys(validateDraftFormValues(values)).length > 0 ||
      !definition ||
      (seed.thumbnail && validateThumbnailFile(seed.thumbnail.blob))
    ) {
      openPage();
      return;
    }

    const ids = { apiPortalId, apiType: target.apiType, apiId: target.apiHandle };
    const run = async () => {
      try {
        await saveDraft.mutateAsync({ ...ids, body: draftFormValuesToInput(values) });
        await saveDefinition.mutateAsync({ ...ids, body: definition });
        if (seed.thumbnail) await saveThumbnail.mutateAsync({ ...ids, file: seed.thumbnail.blob });
        await publish.mutateAsync(ids);
        // Named, as the page may show another API by now.
        notify(
          intl.formatMessage(messages.published, { apiName: values.displayName, portalName }),
          'success',
        );
      } catch (error) {
        await onFailed();
        // Already reported by the global mutation snackbar.
        if (!isApiError(error)) throw error;
      } finally {
        onDone();
      }
    };
    void run();
  });

  return null;
}
