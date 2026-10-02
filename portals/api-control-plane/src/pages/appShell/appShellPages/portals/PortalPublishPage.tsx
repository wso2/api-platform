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
import { Link, useLocation, useNavigate, useParams } from 'react-router-dom';

import {
  REST_API_TYPE,
  useDeprecateRestApiOnApiPortal,
  usePublishRestApiToApiPortal,
  useSaveApiPublicationDraft,
  useSaveApiPublicationDraftDefinition,
  useUnpublishRestApiFromApiPortal,
  type DraftDefinitionDocument,
} from '@/api/resources/apiPublications';
import { isApiError, isErrorCode } from '@/api/core/errors';
import { AppPage } from '@/components/AppPage';
import { ConfirmDialog } from '@/components/ConfirmDialog';
import { useFillScrollArea } from '@/hooks/useFillScrollArea';
import { useFrozenWhile } from '@/hooks/useFrozenWhile';
import { useFormatters } from '@/i18n/useFormatters';
import { useNotifications } from '@/components/Notifications';
import { LoadingState } from '@/components/StateViews';
import { routes } from '@/routes/paths';
import { useConsoleScope } from '@/scope/ConsoleScopeProvider';
import { parseSpecText, type SpecFormat } from '../apis/create/utils/specText';
import { ApiDetailsTab } from './components/ApiDetailsTab';
import { PublicationLoadError } from './components/PublicationLoadError';
import { PublicationVersionCard } from './components/PublicationVersionCard';
import { PublicationVersionToggle } from './components/PublicationVersionToggle';
import { PublishActionsBar } from './components/PublishActionsBar';
import { PublishedSpecificationTab } from './components/PublishedSpecificationTab';
import { SpecificationTab } from './components/SpecificationTab';
import { SubscriptionPlansTab } from './components/SubscriptionPlansTab';
import { usePublishPageData } from './usePublishPageData';
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

/** The action bar's row, held open while it is hidden. */
const ACTION_BAR_HEIGHT = 40;

const messages = defineMessages({
  back: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.PortalPublishPage.back',
    defaultMessage: 'Back to API portals',
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
    defaultMessage: 'Unable to load the publish details.',
  },
  definitionNotAnObject: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.PortalPublishPage.definitionNotAnObject',
    defaultMessage: 'The definition must be a JSON or YAML object.',
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
  draftBannerTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.PortalPublishPage.draftBannerTitle',
    defaultMessage: 'Draft version',
    description: 'Banner over the fields being edited: this is the working copy, not what is live.',
  },
  draftBannerMeta: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.PortalPublishPage.draftBannerMeta',
    defaultMessage: 'v{version} · edited {time}',
    description: 'Banner byline. {version} is user-supplied; {time} is a relative time such as "5 minutes ago".',
  },
  publishedBannerTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.PortalPublishPage.publishedBannerTitle',
    defaultMessage: 'Published version',
    description: 'Banner over the read-only fields showing what is live on the portal.',
  },
  deprecatedBannerTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.PortalPublishPage.deprecatedBannerTitle',
    defaultMessage: 'Deprecated version',
    description: 'Banner over the read-only fields when the live listing is flagged as deprecated.',
  },
  publishedBannerMeta: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.PortalPublishPage.publishedBannerMeta',
    defaultMessage: 'v{version} · updated {time}',
    description: 'Banner byline. {version} is user-supplied; {time} is a relative time such as "2 days ago".',
  },
  draftMissing: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.PortalPublishPage.draftMissing',
    defaultMessage: 'Unable to save the draft. Your changes are still on this page. Try again.',
  },
  draftSaved: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.PortalPublishPage.draftSaved',
    defaultMessage: 'Draft saved.',
  },
  draftSavedDetailsOnly: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.PortalPublishPage.draftSavedDetailsOnly',
    defaultMessage: 'Details saved. The specification has an error and was not saved.',
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
      'This removes the API "{name}" from {portalName}. You can publish it again later.',
  },
  confirmAction: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.PortalPublishPage.confirmAction',
    defaultMessage: 'Confirm',
    description: 'Button on the unpublish and deprecate confirmation dialogs. Verb.',
  },
  confirmInputLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.PortalPublishPage.confirmInputLabel',
    defaultMessage: 'Type "{name}" to confirm',
    description: 'Label for the type-to-confirm field. {name} is the API name; do not translate it.',
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
      'The API "{name}" will be marked as deprecated on {portalName}. It will remain listed.',
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

type PublishTab = 'details' | 'specification' | 'subscriptionPlans';

/** Which action is currently in flight, so the right button (and only that one) shows busy. */
type PendingAction = 'idle' | 'saving' | 'publishing' | 'unpublishing' | 'deprecating';

/**
 * The publish/unpublish/deprecate flow for one API on one API Portal.
 *
 * "API Details", "Specification" and "Subscription Plans" are editable; the
 * other tabs still render disabled. Save Draft and Publish each enforce their
 * own rule: Save Draft always writes `.../draft`, and writes
 * `.../draft/definition` too only when the specification text parses,
 * reporting honestly when it couldn't; Publish requires both saves to succeed
 * before it calls `.../publish`, since the server's publish takes no body and
 * only publishes what the draft already holds.
 *
 * No `ScopeGate`: this page is only reachable from the Portals listing's card,
 * which is already fully API-scoped.
 */
export function PortalPublishPage() {
  return (
    <AppPage hideBreadcrumbs>
      <PortalPublishPageContent />
    </AppPage>
  );
}

function PortalPublishPageContent() {
  const { apiPortalId = '' } = useParams();
  const location = useLocation();
  const navigate = useNavigate();
  const intl = useIntl();
  const { relativeTime } = useFormatters();
  // The page fills the visible area and only its middle scrolls, so switching tabs
  // or opening the editor never moves the header or the action buttons.
  const fill = useFillScrollArea<HTMLDivElement>(MIN_PAGE_HEIGHT);
  const { notify } = useNotifications();
  const { params } = useConsoleScope();
  const orgHandle = params.orgHandle ?? '';
  const projectHandler = params.projectHandler ?? '';
  const apiHandler = params.apiHandler ?? '';

  // Passed by the portal card that linked here; falls back to the handle on a
  // direct link or refresh, where there is no navigation state.
  const portalName = (location.state as { portalName?: string } | null)?.portalName ?? apiPortalId;

  const [tab, setTab] = useState<PublishTab>('details');
  const [viewingPublished, setViewingPublished] = useState(false);
  const data = usePublishPageData(apiPortalId, apiHandler, viewingPublished && tab === 'specification');

  const saveDraftMutation = useSaveApiPublicationDraft();
  const saveDefinitionMutation = useSaveApiPublicationDraftDefinition({ handlesErrors: true });
  const publishMutation = usePublishRestApiToApiPortal();
  const unpublishMutation = useUnpublishRestApiFromApiPortal();
  const deprecateMutation = useDeprecateRestApiOnApiPortal();

  const [values, setValues] = useState<DraftFormValues>(emptyDraftFormValues);
  const [touched, setTouched] = useState<Partial<Record<DraftFormField, boolean>>>({});
  const [definitionText, setDefinitionText] = useState('');
  const [definitionFormat, setDefinitionFormat] = useState<SpecFormat>('json');
  const [definitionParseError, setDefinitionParseError] = useState<string>();
  const [pendingAction, setPendingAction] = useState<PendingAction>('idle');
  // An action saves in steps, each refetching these; they are shown once, when it ends.
  const draft = useFrozenWhile(data.draft, pendingAction !== 'idle');
  const publication = useFrozenWhile(data.publication, pendingAction !== 'idle');
  const [confirmingUnpublish, setConfirmingUnpublish] = useState(false);
  const [confirmingDeprecate, setConfirmingDeprecate] = useState(false);
  const [initialized, setInitialized] = useState(false);

  // Seeds the editor once, when every tier has settled (success or the expected
  // 404), so a later refetch can't overwrite edits in progress.
  useEffect(() => {
    if (initialized || !data.seed) return;
    setValues(data.seed.values);
    setDefinitionText(data.seed.definition?.text ?? '');
    setDefinitionFormat(data.seed.definition?.format ?? 'json');
    setInitialized(true);
  }, [initialized, data.seed]);

  if (data.isLoading) {
    return <LoadingState label={intl.formatMessage(messages.loading)} />;
  }
  if (data.error || !data.api) {
    return (
      <PublicationLoadError
        error={data.error}
        fallbackMessage={intl.formatMessage(messages.errorMessage)}
      />
    );
  }

  const { api } = data;
  const isPublished = Boolean(publication);
  const canDeprecate = publication?.status === 'PUBLISHED';
  // The switch is off while there is nothing live, so an unpublish that lands
  // while the published version is on screen falls back to the draft.
  const showingPublished = viewingPublished && isPublished;
  const errors = validateDraftFormValues(values);
  const errorFor = (field: DraftFormField) => (touched[field] ? errors[field] : undefined);
  const formInvalid = Object.keys(errors).length > 0;

  const markTouched = (field: DraftFormField) =>
    setTouched((current) => ({ ...current, [field]: true }));

  const touchAllFields = () =>
    setTouched({ displayName: true, version: true, productionUrl: true, sandboxUrl: true });

  /**
   * Reads the definition buffer into the object the server expects. A draft
   * is a work in progress — whether it's a genuinely valid, complete OpenAPI
   * document is checked only at publish, not here — so this only blocks the
   * save when the text isn't even parseable JSON/YAML, since that can't be
   * sent as a definition at all.
   */
  const parseDefinition = (): DraftDefinitionDocument | undefined => {
    if (definitionText.trim() === '') return {};
    const result = parseSpecText(definitionText, definitionFormat);
    if (result.status !== 'parsed') {
      setDefinitionParseError(
        result.status === 'malformed' ? result.reason : intl.formatMessage(messages.definitionNotAnObject),
      );
      setTab('specification');
      return undefined;
    }
    setDefinitionParseError(undefined);
    return result.spec;
  };

  /**
   * The definition save 404s when the draft is gone (published from another
   * tab or user in the moment since the details were saved). The server's own
   * text doesn't tell the user what to do, so that case gets a clearer one; the
   * mutation opts out of the global snackbar, so every other failure is reported
   * here with the server's message.
   */
  const reportingMissingDraft = async <T,>(call: Promise<T>): Promise<T> => {
    try {
      return await call;
    } catch (error) {
      if (isApiError(error)) {
        const missing = error.code === 'DRAFT_NOT_FOUND';
        notify(missing ? intl.formatMessage(messages.draftMissing) : error.message, 'error');
      }
      throw error;
    }
  };

  /** Saves the API Details fields alone. False, untouched, when the form itself is invalid. */
  const saveDetails = async (): Promise<boolean> => {
    if (formInvalid) {
      touchAllFields();
      setTab('details');
      return false;
    }
    await saveDraftMutation.mutateAsync({
      apiPortalId,
      apiType: REST_API_TYPE,
      apiId: apiHandler,
      body: draftFormValuesToInput(values),
    });
    return true;
  };

  /**
   * Saves the Specification definition alone. False, untouched, when the
   * buffer doesn't parse — a content PUT would 404 anyway if the draft
   * doesn't exist yet, which `reportingMissingDraft` turns into a clear message.
   */
  const saveDefinition = async (): Promise<boolean> => {
    const definitionDocument = parseDefinition();
    if (!definitionDocument) return false;
    await reportingMissingDraft(
      saveDefinitionMutation.mutateAsync({
        apiPortalId,
        apiType: REST_API_TYPE,
        apiId: apiHandler,
        body: definitionDocument,
      }),
    );
    return true;
  };

  /** Runs one action with its button shown busy; API failures are already reported globally. */
  const runAction = async (action: PendingAction, perform: () => Promise<void>) => {
    setPendingAction(action);
    try {
      await perform();
    } catch (error) {
      rethrowUnreported(error);
    } finally {
      setPendingAction('idle');
    }
  };

  const handleSaveDraft = () =>
    runAction('saving', async () => {
      if (!(await saveDetails())) return;
      const definitionSaved = await saveDefinition();
      notify(
        intl.formatMessage(definitionSaved ? messages.draftSaved : messages.draftSavedDetailsOnly),
        definitionSaved ? 'success' : 'warning',
      );
    });

  // Each terminal action leaves this one portal's page behind for the listing,
  // where the card now reflects the new status — there's nothing left to do
  // on this page once the action the user came here for has gone through.
  const backToPortalsList = () => navigate(routes.apiPortals(orgHandle, projectHandler, apiHandler));

  const handlePublish = () =>
    runAction('publishing', async () => {
      if (!(await saveDetails())) return;
      // Publish requires a saved, parseable definition — never proceed on a
      // partial save, which would publish whatever definition was already
      // there rather than what the user is looking at.
      if (!(await saveDefinition())) return;
      try {
        await publishMutation.mutateAsync({ apiPortalId, apiId: apiHandler });
      } catch (error) {
        // The message itself is already reported by the global snackbar;
        // this only sends the user to where they'd fix it.
        if (isErrorCode(error, 'PUBLICATION_VALIDATION_FAILED')) setTab('specification');
        throw error;
      }
      notify(intl.formatMessage(messages.published, { portalName }), 'success');
      backToPortalsList();
    });

  const confirmUnpublish = () => {
    setConfirmingUnpublish(false);
    return runAction('unpublishing', async () => {
      await unpublishMutation.mutateAsync({ apiPortalId, apiId: apiHandler });
      notify(intl.formatMessage(messages.unpublished, { portalName }), 'success');
      backToPortalsList();
    });
  };

  const confirmDeprecate = () => {
    setConfirmingDeprecate(false);
    return runAction('deprecating', async () => {
      await deprecateMutation.mutateAsync({ apiPortalId, apiId: apiHandler });
      notify(intl.formatMessage(messages.deprecated, { portalName }), 'success');
      backToPortalsList();
    });
  };

  const publishedValues = resolveDraftFormValues(undefined, publication, undefined);
  // A draft that was never saved has nothing to describe, so it gets no banner.
  const banner = showingPublished
    ? {
        meta: intl.formatMessage(messages.publishedBannerMeta, {
          time: relativeTime(publication?.updatedAt),
          version: publication?.version,
        }),
        title: intl.formatMessage(
          canDeprecate ? messages.publishedBannerTitle : messages.deprecatedBannerTitle,
        ),
      }
    : draft && {
        meta: intl.formatMessage(messages.draftBannerMeta, {
          time: relativeTime(draft.updatedAt),
          version: draft.version,
        }),
        title: intl.formatMessage(messages.draftBannerTitle),
      };

  const renderContent = () => {
    if (showingPublished) {
      if (tab === 'details') return <ApiDetailsTab readOnly values={publishedValues} />;
      if (tab === 'subscriptionPlans') return <SubscriptionPlansTab readOnly values={publishedValues} />;
      return (
        <PublishedSpecificationTab
          definition={data.publishedDefinition.definition}
          failed={data.publishedDefinition.failed}
          isLoading={data.publishedDefinition.isLoading}
        />
      );
    }
    if (tab === 'details') {
      return (
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
      );
    }
    if (tab === 'subscriptionPlans') {
      return <SubscriptionPlansTab disabled={pendingAction !== 'idle'} onChange={setValues} values={values} />;
    }
    return (
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
    );
  };

  return (
    <>
      <Box ref={fill.ref} sx={{ display: 'flex', flexDirection: 'column', height: fill.height, minHeight: 0 }}>
        <PageTitle>
          <PageTitle.BackButton component={<Link to={routes.apiPortals(orgHandle, projectHandler, apiHandler)} />}>
            <FormattedMessage {...messages.back} />
          </PageTitle.BackButton>
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
          <Stack
            alignItems="flex-end"
            direction="row"
            justifyContent="space-between"
            sx={{ borderBottom: 1, borderColor: 'divider', flexShrink: 0 }}
          >
            {/* Takes the row's spare width and scrolls within it, so the version toggle is never pushed off. */}
            <Tabs
              allowScrollButtonsMobile
              onChange={(_event, next: PublishTab) => setTab(next)}
              scrollButtons="auto"
              sx={{ flex: 1, minWidth: 0 }}
              value={tab}
              variant="scrollable"
            >
              <Tab label={intl.formatMessage(messages.tabDetails)} value="details" />
              <Tab label={intl.formatMessage(messages.tabSpecification)} value="specification" />
              <Tab label={intl.formatMessage(messages.tabSubscriptionPlans)} value="subscriptionPlans" />
              <Tab disabled label={intl.formatMessage(messages.tabDocumentations)} value="documentations" />
              <Tab disabled label={intl.formatMessage(messages.tabLandingPage)} value="landingPage" />
            </Tabs>
            <PublicationVersionToggle
              disabled={pendingAction !== 'idle'}
              onChange={(version) => setViewingPublished(version === 'published')}
              publishedAvailable={isPublished}
              value={showingPublished ? 'published' : 'draft'}
            />
          </Stack>

          <PublicationVersionCard banner={banner} tone={showingPublished ? 'published' : 'draft'}>
            {renderContent()}
          </PublicationVersionCard>

          {/* Reserves the bar's row while the published version is on screen, so the card doesn't resize. */}
          <Box sx={{ minHeight: ACTION_BAR_HEIGHT }}>
            {!showingPublished && (
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
            )}
          </Box>
        </Stack>
      </Box>

      <ConfirmDialog
        confirmInputLabel={intl.formatMessage(messages.confirmInputLabel, { name: api.displayName })}
        confirmLabel={intl.formatMessage(messages.confirmAction)}
        confirmPhrase={api.displayName}
        destructive
        loading={pendingAction === 'unpublishing'}
        message={intl.formatMessage(messages.unpublishConfirmMessage, { name: api.displayName, portalName })}
        onCancel={() => setConfirmingUnpublish(false)}
        onConfirm={confirmUnpublish}
        open={confirmingUnpublish}
        title={intl.formatMessage(messages.unpublishConfirmTitle)}
      />

      <ConfirmDialog
        confirmColor="warning"
        confirmInputLabel={intl.formatMessage(messages.confirmInputLabel, { name: api.displayName })}
        confirmLabel={intl.formatMessage(messages.confirmAction)}
        confirmPhrase={api.displayName}
        loading={pendingAction === 'deprecating'}
        message={intl.formatMessage(messages.deprecateConfirmMessage, { name: api.displayName, portalName })}
        onCancel={() => setConfirmingDeprecate(false)}
        onConfirm={confirmDeprecate}
        open={confirmingDeprecate}
        title={intl.formatMessage(messages.deprecateConfirmTitle)}
      />
    </>
  );
}
