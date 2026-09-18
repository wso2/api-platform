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

import { useEffect, useState } from 'react';
import { Box, PageTitle, Tab, Tabs } from '@wso2/oxygen-ui';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';
import { Link, useLocation, useParams } from 'react-router-dom';

import {
  useApiPublication,
  useApiPublicationDefinition,
  useApiPublicationDraft,
  useApiPublicationDraftDefinition,
  usePublishRestApiToApiPortal,
  useSaveApiPublicationDraft,
  useSaveApiPublicationDraftDefinition,
  useUnpublishRestApiFromApiPortal,
  type DraftDefinitionDocument,
} from '@/api/resources/apiPublications';
import { useRestApi } from '@/api/resources/restApis';
import { isApiError } from '@/api/core/errors';
import { ConfirmDialog } from '@/components/ConfirmDialog';
import { useNotifications } from '@/components/Notifications';
import { ErrorState, LoadingState } from '@/components/StateViews';
import { routes } from '@/routes/paths';
import { useConsoleScope } from '@/scope/ConsoleScopeProvider';
import { restApiToOpenApiSpec } from '../apis/utils/operationsToSpec';
import { ApiDetailsTab } from './components/ApiDetailsTab';
import { PublishActionsBar } from './components/PublishActionsBar';
import { SpecificationTab } from './components/SpecificationTab';
import {
  draftFormValuesToInput,
  emptyDraftFormValues,
  resolveDraftFormValues,
  validateDraftFormValues,
  type DraftFormField,
  type DraftFormValues,
} from './utils/publicationForm';

const messages = defineMessages({
  back: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.PortalPublishPage.back',
    defaultMessage: 'Back to Available Portals',
  },
  title: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.PortalPublishPage.title',
    defaultMessage: 'Publish to {portalName}',
    description: 'Page heading. {portalName} is the API Portal display name; do not translate it.',
  },
  subtitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.PortalPublishPage.subtitle',
    defaultMessage: '{apiName} · v{version}',
    description: 'Byline under the heading. Both values are user-supplied; do not translate them.',
  },
  loading: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.PortalPublishPage.loading',
    defaultMessage: 'Loading publish details',
  },
  errorMessage: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.PortalPublishPage.errorMessage',
    defaultMessage: 'Unable to load this portal’s draft and publication state',
  },
  tabDetails: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.PortalPublishPage.tabDetails',
    defaultMessage: 'API Details',
  },
  tabSpecification: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.PortalPublishPage.tabSpecification',
    defaultMessage: 'Specification',
  },
  tabSubscriptionPlans: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.PortalPublishPage.tabSubscriptionPlans',
    defaultMessage: 'Subscription Plans',
  },
  tabDocumentations: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.PortalPublishPage.tabDocumentations',
    defaultMessage: 'Documentations',
  },
  tabLandingPage: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.PortalPublishPage.tabLandingPage',
    defaultMessage: 'Landing Page',
  },
  draftSaved: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.PortalPublishPage.draftSaved',
    defaultMessage: 'Draft saved.',
  },
  published: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.PortalPublishPage.published',
    defaultMessage: 'Published to {portalName}.',
  },
  unpublished: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.PortalPublishPage.unpublished',
    defaultMessage: 'Unpublished from {portalName}.',
  },
  unpublishConfirmTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.PortalPublishPage.unpublishConfirmTitle',
    defaultMessage: 'Unpublish this API?',
  },
  unpublishConfirmMessage: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.PortalPublishPage.unpublishConfirmMessage',
    defaultMessage:
      'This removes the live listing from {portalName}. If it still has active subscriptions or API keys, the portal will reject the removal until those are cleared. Either way, the current listing content is kept as a draft, so republishing later is still possible.',
  },
  unpublishConfirmAction: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.PortalPublishPage.unpublishConfirmAction',
    defaultMessage: 'Unpublish',
  },
});

/**
 * `apiType` is hardcoded to `rest-api`, same reasoning as
 * `ApiPortalPublicationsList`: the only API family this build publishes end to
 * end today.
 */
const API_TYPE = 'rest-api';

type Tab = 'details' | 'specification';

/** Which action is currently in flight, so the right button (and only that one) shows busy. */
type PendingAction = 'idle' | 'saving' | 'publishing' | 'unpublishing';

/**
 * The publish/unpublish flow for one API on one API Portal.
 *
 * Alpha scope, per the design this implements: only "API Details" and
 * "Specification" are editable — Subscription Plans, Documentations and
 * Landing Page render as disabled tabs rather than being left out, since
 * they're still on the roadmap, just not this release (contrast the
 * thumbnail/icon control, which is dropped entirely — see `ApiDetailsTab`).
 * Save Draft writes `.../draft` and `.../draft/definition`; Publish writes
 * both of those and then calls `.../publish` in the same click, matching
 * REST_Design.md §7's "client-side sequencing" — the server never accepts a
 * publish with no request body other than what the draft already holds.
 *
 * No `ScopeGate`: like `ApiEditPage`, this page is only reachable from the
 * Portals listing's own card, which is already fully API-scoped.
 */
export function PortalPublishPage() {
  const { apiPortalId = '' } = useParams();
  const location = useLocation();
  const intl = useIntl();
  const { notify } = useNotifications();
  const { params } = useConsoleScope();
  const orgHandle = params.orgHandle ?? '';
  const projectHandler = params.projectHandler ?? '';
  const apiHandler = params.apiHandler ?? '';

  // Handed down from the portal card that linked here, so the heading doesn't
  // need its own fetch just to name the portal. Falls back to the handle for a
  // direct link/refresh, where no navigation state exists.
  const portalName = (location.state as { portalName?: string } | null)?.portalName ?? apiPortalId;

  const apiQuery = useRestApi(apiHandler);
  const draftQuery = useApiPublicationDraft(apiPortalId, API_TYPE, apiHandler);
  const publicationQuery = useApiPublication(apiPortalId, API_TYPE, apiHandler);
  const draftDefinitionQuery = useApiPublicationDraftDefinition(apiPortalId, API_TYPE, apiHandler);
  const publicationDefinitionQuery = useApiPublicationDefinition(apiPortalId, API_TYPE, apiHandler);

  const saveDraftMutation = useSaveApiPublicationDraft();
  const saveDefinitionMutation = useSaveApiPublicationDraftDefinition();
  const publishMutation = usePublishRestApiToApiPortal();
  const unpublishMutation = useUnpublishRestApiFromApiPortal();

  const [tab, setTab] = useState<Tab>('details');
  const [values, setValues] = useState<DraftFormValues>(emptyDraftFormValues);
  const [touched, setTouched] = useState<Partial<Record<DraftFormField, boolean>>>({});
  const [definitionText, setDefinitionText] = useState('');
  const [definitionParseError, setDefinitionParseError] = useState<string>();
  const [pendingAction, setPendingAction] = useState<PendingAction>('idle');
  const [confirmingUnpublish, setConfirmingUnpublish] = useState(false);
  const [initialized, setInitialized] = useState(false);

  const initialLoadPending =
    apiQuery.isPending ||
    draftQuery.isPending ||
    publicationQuery.isPending ||
    draftDefinitionQuery.isPending ||
    publicationDefinitionQuery.isPending;

  // Seeds the form from REST_Design.md §6's read chain exactly once, the
  // moment every tier has settled (success or the expected 404) — never
  // again, so a background refetch (or the invalidation a save triggers)
  // can't clobber edits already in progress.
  useEffect(() => {
    if (initialized || initialLoadPending) return;

    setValues(resolveDraftFormValues(draftQuery.data, publicationQuery.data, apiQuery.data));

    const definitionDocument: DraftDefinitionDocument | undefined =
      draftDefinitionQuery.data ??
      publicationDefinitionQuery.data ??
      (apiQuery.data ? restApiToOpenApiSpec(apiQuery.data) : undefined);
    setDefinitionText(definitionDocument ? JSON.stringify(definitionDocument, null, 2) : '');

    setInitialized(true);
  }, [
    initialized,
    initialLoadPending,
    apiQuery.data,
    draftQuery.data,
    publicationQuery.data,
    draftDefinitionQuery.data,
    publicationDefinitionQuery.data,
  ]);

  // A 404 on the draft/publication/definition tiers is an expected "nothing
  // saved here yet", not a failure — only a genuinely unexpected error (or the
  // API itself not resolving) is worth an error screen.
  const unexpectedError =
    apiQuery.error ??
    [draftQuery, publicationQuery, draftDefinitionQuery, publicationDefinitionQuery].find(
      (query) => isApiError(query.error) && !query.error.isNotFound,
    )?.error;

  if (initialLoadPending) {
    return <LoadingState label={intl.formatMessage(messages.loading)} />;
  }
  if (unexpectedError || !apiQuery.data) {
    return <ErrorState message={intl.formatMessage(messages.errorMessage)} />;
  }

  const api = apiQuery.data;
  const isPublished = Boolean(publicationQuery.data);
  const errors = validateDraftFormValues(values);
  const errorFor = (field: DraftFormField) => (touched[field] ? errors[field] : undefined);
  const formInvalid = Object.keys(errors).length > 0;

  const markTouched = (field: DraftFormField) =>
    setTouched((current) => ({ ...current, [field]: true }));

  const touchAllFields = () =>
    setTouched({ displayName: true, version: true, productionUrl: true, sandboxUrl: true });

  /** Parses the definition buffer, surfacing a malformed one on its own tab rather than failing silently. */
  const parseDefinition = (): DraftDefinitionDocument | undefined => {
    try {
      const parsed: unknown = definitionText.trim() === '' ? {} : JSON.parse(definitionText);
      setDefinitionParseError(undefined);
      return parsed as DraftDefinitionDocument;
    } catch (error) {
      setDefinitionParseError(error instanceof Error ? error.message : String(error));
      setTab('specification');
      return undefined;
    }
  };

  /** Details, then definition — a content PUT 404s if the draft doesn't exist yet (REST_Design.md §7). */
  const saveDraft = async (): Promise<boolean> => {
    if (formInvalid) {
      touchAllFields();
      setTab('details');
      return false;
    }
    const definitionDocument = parseDefinition();
    if (!definitionDocument) return false;

    await saveDraftMutation.mutateAsync({
      apiPortalId,
      apiType: API_TYPE,
      apiId: apiHandler,
      body: draftFormValuesToInput(values),
    });
    await saveDefinitionMutation.mutateAsync({
      apiPortalId,
      apiType: API_TYPE,
      apiId: apiHandler,
      body: definitionDocument,
    });
    return true;
  };

  const handleSaveDraft = async () => {
    setPendingAction('saving');
    try {
      if (await saveDraft()) {
        notify(intl.formatMessage(messages.draftSaved), 'success');
      }
    } finally {
      setPendingAction('idle');
    }
  };

  const handlePublish = async () => {
    setPendingAction('publishing');
    try {
      // Client-side sequencing per REST_Design.md §7: the draft PUT(s) go
      // first — same as Save Draft — then the bodyless publish call.
      if (!(await saveDraft())) return;
      await publishMutation.mutateAsync({ apiPortalId, apiId: apiHandler });
      notify(intl.formatMessage(messages.published, { portalName }), 'success');
    } finally {
      setPendingAction('idle');
    }
  };

  const confirmUnpublish = async () => {
    setConfirmingUnpublish(false);
    setPendingAction('unpublishing');
    try {
      await unpublishMutation.mutateAsync({ apiPortalId, apiId: apiHandler });
      notify(intl.formatMessage(messages.unpublished, { portalName }), 'success');
    } finally {
      setPendingAction('idle');
    }
  };

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', minHeight: '100%' }}>
      <PageTitle>
        <Link to={routes.apiPortals(orgHandle, projectHandler, apiHandler)}>
          <PageTitle.BackButton>
            <FormattedMessage {...messages.back} />
          </PageTitle.BackButton>
        </Link>
        <PageTitle.Header>
          <FormattedMessage {...messages.title} values={{ portalName }} />
        </PageTitle.Header>
        <PageTitle.SubHeader>
          <FormattedMessage
            {...messages.subtitle}
            values={{ apiName: api.displayName, version: api.version }}
          />
        </PageTitle.SubHeader>
      </PageTitle>

      <Box sx={{ borderBottom: 1, borderColor: 'divider' }}>
        <Tabs onChange={(_event, next: Tab) => setTab(next)} value={tab}>
          <Tab label={intl.formatMessage(messages.tabDetails)} value="details" />
          <Tab label={intl.formatMessage(messages.tabSpecification)} value="specification" />
          {/* Disabled, not omitted — still on the roadmap, just not this release. */}
          <Tab disabled label={intl.formatMessage(messages.tabSubscriptionPlans)} value="subscriptionPlans" />
          <Tab disabled label={intl.formatMessage(messages.tabDocumentations)} value="documentations" />
          <Tab disabled label={intl.formatMessage(messages.tabLandingPage)} value="landingPage" />
        </Tabs>
      </Box>

      <Box sx={{ flex: 1, py: 3 }}>
        {tab === 'details' ? (
          <ApiDetailsTab
            disabled={pendingAction !== 'idle'}
            errors={{
              displayName: errorFor('displayName'),
              version: errorFor('version'),
              productionUrl: errorFor('productionUrl'),
              sandboxUrl: errorFor('sandboxUrl'),
            }}
            onBlurField={markTouched}
            onChange={setValues}
            values={values}
          />
        ) : (
          <SpecificationTab
            disabled={pendingAction !== 'idle'}
            onChange={(text) => {
              setDefinitionText(text);
              if (definitionParseError) setDefinitionParseError(undefined);
            }}
            parseError={definitionParseError}
            text={definitionText}
          />
        )}
      </Box>

      <PublishActionsBar
        isPublished={isPublished}
        onPublish={handlePublish}
        onSaveDraft={handleSaveDraft}
        onUnpublish={() => setConfirmingUnpublish(true)}
        publishing={pendingAction === 'publishing'}
        savingDraft={pendingAction === 'saving'}
        unpublishing={pendingAction === 'unpublishing'}
      />

      <ConfirmDialog
        confirmLabel={intl.formatMessage(messages.unpublishConfirmAction)}
        destructive
        loading={pendingAction === 'unpublishing'}
        message={intl.formatMessage(messages.unpublishConfirmMessage, { portalName })}
        onCancel={() => setConfirmingUnpublish(false)}
        onConfirm={confirmUnpublish}
        open={confirmingUnpublish}
        title={intl.formatMessage(messages.unpublishConfirmTitle)}
      />
    </Box>
  );
}
