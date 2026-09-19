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
import { Box, PageTitle, Stack, Tab, Tabs } from '@wso2/oxygen-ui';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';
import { Link, useLocation, useParams } from 'react-router-dom';

import {
  useApiPublication,
  useApiPublicationDefinition,
  useApiPublicationDraft,
  useApiPublicationDraftDefinition,
  useDeprecateRestApiOnApiPortal,
  usePublishRestApiToApiPortal,
  useSaveApiPublicationDraft,
  useSaveApiPublicationDraftDefinition,
  useUnpublishRestApiFromApiPortal,
  type DraftDefinitionDocument,
} from '@/api/resources/apiPublications';
import { useRestApi, useRestApiOpenApi } from '@/api/resources/restApis';
import { isApiError } from '@/api/core/errors';
import { ConfirmDialog } from '@/components/ConfirmDialog';
import { useFillScrollArea } from '@/hooks/useFillScrollArea';
import { useNotifications } from '@/components/Notifications';
import { ErrorState, LoadingState } from '@/components/StateViews';
import { routes } from '@/routes/paths';
import { useConsoleScope } from '@/scope/ConsoleScopeProvider';
import { ApiDetailsTab } from './components/ApiDetailsTab';
import { PublishActionsBar } from './components/PublishActionsBar';
import { parseSpecText, serializeSpec, type SpecFormat } from '../apis/create/utils/specText';
import { SpecificationTab } from './components/SpecificationTab';
import {
  draftFormValuesToInput,
  emptyDraftFormValues,
  resolveDraftFormValues,
  validateDraftFormValues,
  type DraftFormField,
  type DraftFormValues,
} from './utils/publicationForm';

/** Below this the window is too short to fit the page without scrolling, so it scrolls instead. */
const MIN_PAGE_HEIGHT = 420;

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
  definitionNotAnObject: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.PortalPublishPage.definitionNotAnObject',
    defaultMessage: 'The definition must be an object, not a list or a single value.',
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
    defaultMessage: 'Documentation',
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
  deprecated: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.PortalPublishPage.deprecated',
    defaultMessage: 'Deprecated on {portalName}.',
  },
  deprecateConfirmTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.PortalPublishPage.deprecateConfirmTitle',
    defaultMessage: 'Deprecate this API?',
  },
  deprecateConfirmMessage: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.PortalPublishPage.deprecateConfirmMessage',
    defaultMessage:
      'This marks the live listing on {portalName} as deprecated. It stays visible there with a deprecated flag, and your current draft is not changed. You can unpublish it or publish again later.',
  },
  deprecateConfirmAction: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.PortalPublishPage.deprecateConfirmAction',
    defaultMessage: 'Deprecate',
  },
});

/**
 * A failed API call is already reported by the global mutation snackbar, so the
 * action handlers only need to stop it from surfacing again as an unhandled
 * rejection. Anything that is not an `ApiError` is a real bug and still throws.
 */
const rethrowUnreported = (error: unknown): void => {
  if (!isApiError(error)) throw error;
};

/**
 * `apiType` is hardcoded to `rest-api`, same reasoning as
 * `ApiPortalPublicationsList`: the only API family this build publishes end to
 * end today.
 */
const API_TYPE = 'rest-api';

type PublishTab = 'details' | 'specification';

/** Which action is currently in flight, so the right button (and only that one) shows busy. */
type PendingAction = 'idle' | 'saving' | 'publishing' | 'unpublishing' | 'deprecating';

/** A stored definition as the editor shows it. */
type StoredDefinition = { format: SpecFormat; text: string };

/**
 * A stored definition's serialization: the content type the server labelled it
 * with, or, where there isn't one (the API's own spec), what the text looks like.
 */
const formatOf = (text: string, contentType = ''): SpecFormat => {
  if (/ya?ml/i.test(contentType)) return 'yaml';
  if (/json/i.test(contentType)) return 'json';
  return text.trimStart().startsWith('{') ? 'json' : 'yaml';
};

/**
 * Reads a definition delivered as text: the draft and publication definitions
 * come back in whichever serialization they were saved in, and
 * `GET /rest-apis/{id}/openapi` (`useRestApiOpenApi`) returns the raw spec. It
 * is shown in that same format — YAML as stored, JSON pretty-printed — so what
 * the user opens is what was saved. Text that doesn't read as an object is
 * treated as "nothing to pre-fill from".
 */
const readStoredDefinition = (text: string, contentType?: string): StoredDefinition | undefined => {
  const format = formatOf(text, contentType);
  const parsed = parseSpecText(text, format);
  if (parsed.status !== 'parsed') return undefined;
  return { format, text: format === 'json' ? serializeSpec(parsed.spec, 'json') : text };
};

/**
 * The publish/unpublish/deprecate flow for one API on one API Portal.
 *
 * Alpha scope, per the design this implements: only "API Details" and
 * "Specification" are editable — Subscription Plans, Documentation and
 * Landing Page render as disabled tabs rather than being left out, since
 * they're still on the roadmap, just not this release (contrast the
 * thumbnail/icon control, which is dropped entirely — see `ApiDetailsTab`).
 * Save Draft writes `.../draft` and `.../draft/definition`; Publish writes
 * both of those and then calls `.../publish` in the same click — the client
 * sequences the calls because the server's publish takes no request body and
 * only ever publishes what the draft already holds.
 *
 * No `ScopeGate`: like `ApiEditPage`, this page is only reachable from the
 * Portals listing's own card, which is already fully API-scoped.
 */
export function PortalPublishPage() {
  const { apiPortalId = '' } = useParams();
  const location = useLocation();
  const intl = useIntl();
  // The page fills the visible area and only its middle scrolls, so switching tabs
  // or opening the editor never moves the header or the action buttons.
  const fill = useFillScrollArea<HTMLDivElement>(MIN_PAGE_HEIGHT);
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
  // `draft` and `publication` fetch in parallel, not as a fallback chain —
  // `publication` isn't only a pre-fill fallback here, it's also what decides
  // whether Unpublish is enabled, which is needed regardless of whether a
  // draft exists (a portal can be live *and* have an in-progress draft edit
  // at once). Deferring it behind "no draft" would silently disable Unpublish
  // on exactly that combination.
  const draftQuery = useApiPublicationDraft(apiPortalId, API_TYPE, apiHandler);
  const publicationQuery = useApiPublication(apiPortalId, API_TYPE, apiHandler);

  // The Specification tab's three definition tiers, by contrast, exist only
  // to pre-fill that one tab — nothing else reads them — so each one is
  // fetched only once the tier before it is confirmed absent, rather than all
  // three firing in parallel on every visit. Passing `undefined` for the API
  // handle is what keeps a not-yet-relevant tier's query disabled (every hook
  // here gates on its id argument being defined).
  const draftDefinitionQuery = useApiPublicationDraftDefinition(apiPortalId, API_TYPE, apiHandler);
  const draftDefinitionAbsent = isApiError(draftDefinitionQuery.error) && draftDefinitionQuery.error.isNotFound;

  const publicationDefinitionQuery = useApiPublicationDefinition(
    apiPortalId,
    API_TYPE,
    draftDefinitionAbsent ? apiHandler : undefined,
  );
  const publicationDefinitionAbsent =
    draftDefinitionAbsent &&
    isApiError(publicationDefinitionQuery.error) &&
    publicationDefinitionQuery.error.isNotFound;

  // Last fallback tier: the API's own real stored definition
  // (`GET /rest-apis/{id}/openapi`) — not a reconstruction. 404s when nothing
  // has ever been uploaded for this API.
  const apiOpenApiQuery = useRestApiOpenApi(publicationDefinitionAbsent ? apiHandler : undefined);

  const saveDraftMutation = useSaveApiPublicationDraft();
  const saveDefinitionMutation = useSaveApiPublicationDraftDefinition();
  const publishMutation = usePublishRestApiToApiPortal();
  const unpublishMutation = useUnpublishRestApiFromApiPortal();
  const deprecateMutation = useDeprecateRestApiOnApiPortal();

  const [tab, setTab] = useState<PublishTab>('details');
  const [values, setValues] = useState<DraftFormValues>(emptyDraftFormValues);
  const [touched, setTouched] = useState<Partial<Record<DraftFormField, boolean>>>({});
  const [definitionText, setDefinitionText] = useState('');
  const [definitionFormat, setDefinitionFormat] = useState<SpecFormat>('json');
  const [definitionParseError, setDefinitionParseError] = useState<string>();
  const [pendingAction, setPendingAction] = useState<PendingAction>('idle');
  const [confirmingUnpublish, setConfirmingUnpublish] = useState(false);
  const [confirmingDeprecate, setConfirmingDeprecate] = useState(false);
  const [initialized, setInitialized] = useState(false);

  // A disabled query's own `isPending` is permanently `true` — it never runs,
  // so it never resolves to success/error (see `useApiPublicationDraft`'s own
  // note on `isPending` vs `isLoading`). A tier that isn't enabled yet must
  // only block the page while it's actually still a candidate to run: once
  // its predecessor is confirmed absent it's genuinely in flight and worth
  // waiting on; before that, it would just hold the spinner up forever.
  const initialLoadPending =
    apiQuery.isPending ||
    draftQuery.isPending ||
    publicationQuery.isPending ||
    draftDefinitionQuery.isPending ||
    (draftDefinitionAbsent && publicationDefinitionQuery.isPending) ||
    (publicationDefinitionAbsent && apiOpenApiQuery.isPending);

  // Seeds the form from the draft, publication and API tiers exactly once, the
  // moment every tier has settled (success or the expected 404) — never
  // again, so a background refetch (or the invalidation a save triggers)
  // can't clobber edits already in progress.
  useEffect(() => {
    if (initialized || initialLoadPending) return;

    setValues(resolveDraftFormValues(draftQuery.data, publicationQuery.data, apiQuery.data));

    const stored =
      (draftDefinitionQuery.data &&
        readStoredDefinition(draftDefinitionQuery.data.text, draftDefinitionQuery.data.contentType)) ??
      (publicationDefinitionQuery.data &&
        readStoredDefinition(publicationDefinitionQuery.data.text, publicationDefinitionQuery.data.contentType)) ??
      (apiOpenApiQuery.data ? readStoredDefinition(apiOpenApiQuery.data.content) : undefined);
    setDefinitionText(stored?.text ?? '');
    setDefinitionFormat(stored?.format ?? 'json');

    setInitialized(true);
  }, [
    initialized,
    initialLoadPending,
    apiQuery.data,
    draftQuery.data,
    publicationQuery.data,
    draftDefinitionQuery.data,
    publicationDefinitionQuery.data,
    apiOpenApiQuery.data,
  ]);

  // A 404 on the draft/publication/definition/openapi tiers is an expected
  // "nothing saved here yet", not a failure — only a genuinely unexpected
  // error (or the API itself not resolving) is worth an error screen.
  const unexpectedError =
    apiQuery.error ??
    [
      draftQuery,
      publicationQuery,
      draftDefinitionQuery,
      publicationDefinitionQuery,
      apiOpenApiQuery,
    ].find((query) => isApiError(query.error) && !query.error.isNotFound)?.error;

  if (initialLoadPending) {
    return <LoadingState label={intl.formatMessage(messages.loading)} />;
  }
  if (unexpectedError || !apiQuery.data) {
    return <ErrorState message={intl.formatMessage(messages.errorMessage)} />;
  }

  const api = apiQuery.data;
  // A refetch that 404s (PUBLICATION_NOT_FOUND after an unpublish) keeps the
  // previous successful `data` alongside the error, so the 404 itself decides
  // that the listing is gone. Any other failure (5xx, network) says nothing about
  // the listing, so the last known state is kept.
  const publicationGone = isApiError(publicationQuery.error) && publicationQuery.error.isNotFound;
  const livePublication = publicationGone ? undefined : publicationQuery.data;
  const isPublished = Boolean(livePublication);
  const canDeprecate = livePublication?.status === 'PUBLISHED';
  const errors = validateDraftFormValues(values);
  const errorFor = (field: DraftFormField) => (touched[field] ? errors[field] : undefined);
  const formInvalid = Object.keys(errors).length > 0;

  const markTouched = (field: DraftFormField) =>
    setTouched((current) => ({ ...current, [field]: true }));

  const touchAllFields = () =>
    setTouched({ displayName: true, version: true, productionUrl: true, sandboxUrl: true });

  /** Parses the definition buffer, surfacing a malformed one on its own tab rather than failing silently. */
  const parseDefinition = (): DraftDefinitionDocument | undefined => {
    if (definitionText.trim() === '') {
      setDefinitionParseError(undefined);
      return {};
    }
    const result = parseSpecText(definitionText, definitionFormat);
    if (result.status === 'parsed') {
      setDefinitionParseError(undefined);
      return result.spec;
    }
    setDefinitionParseError(
      result.status === 'malformed' ? result.reason : intl.formatMessage(messages.definitionNotAnObject),
    );
    setTab('specification');
    return undefined;
  };

  /** Details, then definition — a content PUT 404s if the draft doesn't exist yet. */
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
    } catch (error) {
      rethrowUnreported(error);
    } finally {
      setPendingAction('idle');
    }
  };

  const handlePublish = async () => {
    setPendingAction('publishing');
    try {
      // The draft PUT(s) go first — same as Save Draft — then the bodyless
      // publish call.
      if (!(await saveDraft())) return;
      await publishMutation.mutateAsync({ apiPortalId, apiId: apiHandler });
      notify(intl.formatMessage(messages.published, { portalName }), 'success');
    } catch (error) {
      rethrowUnreported(error);
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
    } catch (error) {
      rethrowUnreported(error);
    } finally {
      setPendingAction('idle');
    }
  };

  const confirmDeprecate = async () => {
    setConfirmingDeprecate(false);
    setPendingAction('deprecating');
    try {
      await deprecateMutation.mutateAsync({ apiPortalId, apiId: apiHandler });
      notify(intl.formatMessage(messages.deprecated, { portalName }), 'success');
    } catch (error) {
      rethrowUnreported(error);
    } finally {
      setPendingAction('idle');
    }
  };

  return (
    <>
      <Box ref={fill.ref} sx={{ display: 'flex', flexDirection: 'column', height: fill.height, minHeight: 0 }}>
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

        <Stack spacing={3} sx={{ flex: 1, minHeight: 0 }}>
          <Box sx={{ display: 'flex', flex: 1, flexDirection: 'column', minHeight: 0 }}>
            <Box sx={{ borderBottom: 1, borderColor: 'divider', flexShrink: 0 }}>
              <Tabs onChange={(_event, next: PublishTab) => setTab(next)} value={tab}>
                <Tab label={intl.formatMessage(messages.tabDetails)} value="details" />
                <Tab label={intl.formatMessage(messages.tabSpecification)} value="specification" />
                {/* Disabled, not omitted — still on the roadmap, just not this release. */}
                <Tab disabled label={intl.formatMessage(messages.tabSubscriptionPlans)} value="subscriptionPlans" />
                <Tab disabled label={intl.formatMessage(messages.tabDocumentations)} value="documentations" />
                <Tab disabled label={intl.formatMessage(messages.tabLandingPage)} value="landingPage" />
              </Tabs>
            </Box>

            <Box sx={{ flex: 1, minHeight: 0, overflowY: 'auto', pt: 3 }}>
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
                  format={definitionFormat}
                  onFormatChange={setDefinitionFormat}
                  onChange={(text) => {
                    setDefinitionText(text);
                    if (definitionParseError) setDefinitionParseError(undefined);
                  }}
                  parseError={definitionParseError}
                  text={definitionText}
                />
              )}
            </Box>
          </Box>

          <PublishActionsBar
            canDeprecate={canDeprecate}
            deprecating={pendingAction === 'deprecating'}
            isPublished={isPublished}
            onDeprecate={() => setConfirmingDeprecate(true)}
            onPublish={handlePublish}
            onSaveDraft={handleSaveDraft}
            onUnpublish={() => setConfirmingUnpublish(true)}
            publishing={pendingAction === 'publishing'}
            savingDraft={pendingAction === 'saving'}
            unpublishing={pendingAction === 'unpublishing'}
          />
        </Stack>
      </Box>

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

      <ConfirmDialog
        confirmLabel={intl.formatMessage(messages.deprecateConfirmAction)}
        loading={pendingAction === 'deprecating'}
        message={intl.formatMessage(messages.deprecateConfirmMessage, { portalName })}
        onCancel={() => setConfirmingDeprecate(false)}
        onConfirm={confirmDeprecate}
        open={confirmingDeprecate}
        title={intl.formatMessage(messages.deprecateConfirmTitle)}
      />
    </>
  );
}
