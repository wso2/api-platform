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

import React, { useCallback, useEffect, useState, type ReactNode } from 'react';
import { Link, useLocation, useNavigate, useParams } from 'react-router-dom';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';
import { ChevronLeft } from '@wso2/oxygen-ui-icons-react';
import { Alert, alpha, Box, Button, PageTitle, Stack, Tab, Tabs } from '@wso2/oxygen-ui';

import { mcpProxiesApis } from '../../../../apis/MCP/mcpProxiesApis';
import { SCOPES } from '../../../../auth/permissions';
import ErrorAlert from '../../../../Components/common/ErrorAlert';
import { useAppAuth } from '../../../../contexts/AppAuthContext';
import { formatRelativeTime } from '../../../../contexts/llmProvider';
import useAIWorkspaceSnackbar from '../../../../hooks/aiWorkspaceSnackbar';
import { useFillScrollArea } from '../../../../hooks/useFillScrollArea';
import { useFrozenWhile } from '../../../../hooks/useFrozenWhile';
import { useMcpProxyDocuments } from '../../../../hooks/useMcpProxyDocuments';
import { useObjectUrl } from '../../../../hooks/useObjectUrl';
import { usePublishProject } from '../../../../hooks/usePublishProject';
import { useSubscriptionPlans } from '../../../../hooks/useSubscriptionPlans';
import { PLATFORM_API_BASE_URL } from '../../../../paths';
import { getErrorCode, getErrorMessage, getFieldErrors } from '../../../../utils/apiError';

import { AppPage } from './AppPage';
import { CapabilitiesTab, type CapabilitiesTabProps } from './CapabilitiesTab';
import { ConfirmDialog } from './ConfirmDialog';
import { DocumentsTab } from './DocumentsTab';
import { LandingPageTab } from './LandingPageTab';
import { buildLandingPageStarter } from './landingPageStarter';
import { McpProxyDetailsTab } from './McpProxyDetailsTab';
import {
  deletedDocumentsOf,
  deletedPlansOf,
  definitionToText,
  documentNamesOf,
  draftFormValuesToInput,
  editedDocIdsOf,
  editedPlanIdsOf,
  emptyDraftFormValues,
  parseDefinitionText,
  planNamesOf,
  resolveDraftFormValues,
  sameSelection,
  validateDraftFormValues,
  validateThumbnailFile,
  type DraftFormField,
  type DraftFormValues,
} from './mcpPublishForm';
import { PublicationVersionCard, type PublicationVersionTone } from './PublicationVersionCard';
import { PublicationVersionToggle } from './PublicationVersionToggle';
import { PublishActionsBar } from './PublishActionsBar';
import { ErrorState, LoadingState } from './PublishStates';
import { SubscriptionPlansTab } from './SubscriptionPlansTab';
import { usePublishAssets } from './usePublishAssets';
import { usePublishPageData } from './usePublishPageData';

const messages = defineMessages({
  draftNotFound: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.page.draftNotFound',
    defaultMessage: 'Unable to save the draft. Your changes are still on this page. Try again.',
  },
  loading: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.page.loading',
    defaultMessage: 'Loading publish details',
  },
  loadFailed: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.page.loadFailed',
    defaultMessage: 'Unable to load the publish details.',
  },
  saveFailed: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.page.saveFailed',
    defaultMessage: 'Failed to save the draft.',
  },
  draftSaved: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.page.draftSaved',
    defaultMessage: 'Draft saved.',
  },
  assetsUnreadable: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.page.assetsUnreadable',
    defaultMessage:
      'The thumbnail or landing page could not be loaded, so saving now would remove it from the MCP Hub listing. Reload the page to try again.',
  },
  publishFailed: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.page.publishFailed',
    defaultMessage: 'Failed to publish the MCP proxy.',
  },
  published: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.page.published',
    defaultMessage: 'Published to {hub}.',
  },
  unpublishFailed: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.page.unpublishFailed',
    defaultMessage: 'Failed to unpublish the MCP proxy.',
  },
  unpublished: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.page.unpublished',
    defaultMessage: 'Unpublished from {hub}.',
  },
  deprecateFailed: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.page.deprecateFailed',
    defaultMessage: 'Failed to deprecate the MCP proxy.',
  },
  deprecated: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.page.deprecated',
    defaultMessage: 'Deprecated on {hub}.',
  },
  edited: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.page.edited',
    defaultMessage: 'Edited {time}',
  },
  updated: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.page.updated',
    defaultMessage: 'Updated {time}',
  },
  deprecatedAt: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.page.deprecatedAt',
    defaultMessage: 'Deprecated {time}',
  },
  loadingPublishedSpec: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.page.loadingPublishedSpec',
    defaultMessage: 'Loading the published capabilities',
  },
  publishedSpecFailed: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.page.publishedSpecFailed',
    defaultMessage: 'Unable to load the published capabilities.',
  },
  tabDetails: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.page.tabDetails',
    defaultMessage: 'MCP Proxy Details',
  },
  tabCapabilities: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.page.tabCapabilities',
    defaultMessage: 'Capabilities',
  },
  tabPlans: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.page.tabPlans',
    defaultMessage: 'Subscription Plans',
  },
  tabDocuments: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.page.tabDocuments',
    defaultMessage: 'Documentation',
  },
  tabLanding: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.page.tabLanding',
    defaultMessage: 'Landing Page',
  },
  typeToConfirm: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.page.typeToConfirm',
    defaultMessage: 'Type "{name}" to confirm',
  },
  unpublishMessage: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.page.unpublishMessage',
    defaultMessage: 'This removes the MCP proxy "{name}" from {hub}. You can publish it again later.',
  },
  unpublishTitle: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.page.unpublishTitle',
    defaultMessage: 'Unpublish this MCP proxy?',
  },
  deprecateMessage: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.page.deprecateMessage',
    defaultMessage: 'The MCP proxy "{name}" will be marked as deprecated on {hub}. It will remain listed.',
  },
  deprecateTitle: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.page.deprecateTitle',
    defaultMessage: 'Deprecate this MCP proxy?',
  },
  unsavedChanges: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.page.unsavedChanges',
    defaultMessage: 'Unsaved changes',
  },
  editedUnsaved: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.page.editedUnsaved',
    defaultMessage: 'Edited {time} · {unsaved}',
  },
  back: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.page.back',
    defaultMessage: 'Back to MCP Hubs',
  },
  title: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.page.title',
    defaultMessage: 'Publish {name}',
  },
  subtitle: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.page.subtitle',
    defaultMessage: 'To {hub}',
  },
});

/** Below this the window is too short to fit the page without scrolling, so it scrolls instead. */
const MIN_PAGE_HEIGHT = 420;

/** The action bar's row, held open while it is hidden. */
const ACTION_BAR_HEIGHT = 40;

/** Drops from a selection the ids the draft can't offer. */
const withoutUnusableIds =
  (field: 'docIds' | 'subscriptionPlanIds', isUsable: (id: string) => boolean) =>
  (current: DraftFormValues): DraftFormValues => ({ ...current, [field]: current[field].filter(isUsable) });

/** The fields that show a validation message. */
const FORM_FIELDS: DraftFormField[] = ['displayName', 'version', 'productionUrl', 'sandboxUrl'];

type PublishTab = 'details' | 'capabilities' | 'subscriptionPlans' | 'documentations' | 'landingPage';

/** The action in flight, so only its button shows busy. */
type PendingAction = 'idle' | 'saving' | 'publishing' | 'unpublishing' | 'deprecating';

/** A highlight on the text itself: an inline span's padding can't make the banner taller. */
const unsavedChanges = (
  <Box
    component="span"
    sx={(theme) => ({
      bgcolor: alpha(theme.palette.warning.main, 0.16),
      borderRadius: 1,
      color: 'warning.dark',
      fontWeight: 600,
      mx: 0.5,
      px: 1,
      py: 0.25,
    })}
  >
    <FormattedMessage {...messages.unsavedChanges} />
  </Box>
);

const bold = (text: ReactNode) => (
  <Box component="span" sx={{ fontWeight: 700 }}>
    {text}
  </Box>
);

/**
 * Publishing one MCP proxy to one MCP Hub. Publish takes no body and publishes what the draft
 * holds, so every part is saved first.
 */
export default function McpPublishHubPage(): React.JSX.Element {
  const intl = useIntl();
  const { serverId = '', apiPortalId = '' } = useParams<{ serverId: string; apiPortalId: string }>();
  const location = useLocation();
  const navigate = useNavigate();
  const showSnackbar = useAIWorkspaceSnackbar();
  const { hasPermission } = useAppAuth();
  const { organizationId, publishPath } = usePublishProject();

  // Only the middle scrolls, so switching tabs never moves the header or the actions.
  const fill = useFillScrollArea<HTMLDivElement>(MIN_PAGE_HEIGHT);
  const hubsPath = publishPath(`/${encodeURIComponent(serverId)}`);

  const [tab, setTab] = useState<PublishTab>('details');
  // Which version the user picked; until they pick, the live version if there is one, else the draft.
  const [chosenView, setChosenView] = useState<PublicationVersionTone>();
  const data = usePublishPageData(apiPortalId, serverId, organizationId, chosenView !== 'draft' && tab === 'capabilities');

  // From the hub card that linked here; a direct link or refresh falls back to the hub's name, then its handle.
  const hubName = (location.state as { hubName?: string } | null)?.hubName ?? data.portal?.name ?? apiPortalId;

  const [values, setValues] = useState<DraftFormValues>(emptyDraftFormValues);
  const [touched, setTouched] = useState<Partial<Record<DraftFormField, boolean>>>({});
  const [definitionText, setDefinitionText] = useState('');
  const assets = usePublishAssets(apiPortalId, serverId);
  const thumbnailUrl = useObjectUrl(assets.thumbnail);
  const publishedThumbnailUrl = useObjectUrl(data.publishedThumbnail);
  // What the server holds, so unsaved edits can be told apart.
  const [saved, setSaved] = useState({ definitionText: '', values: emptyDraftFormValues });
  const [pendingAction, setPendingAction] = useState<PendingAction>('idle');
  const busy = pendingAction !== 'idle';
  // An action changes these in steps; they are shown once, when it ends.
  const draft = useFrozenWhile(data.draft, busy);
  const publication = useFrozenWhile(data.publication, busy);
  const [confirmingUnpublish, setConfirmingUnpublish] = useState(false);
  const [confirmingDeprecate, setConfirmingDeprecate] = useState(false);
  const [initialized, setInitialized] = useState(false);

  // Read once its tab is open, or a selection is held to check it against.
  const plans = useSubscriptionPlans(tab === 'subscriptionPlans' || values.subscriptionPlanIds.length > 0);
  const documents = useMcpProxyDocuments(serverId, tab === 'documentations' || values.docIds.length > 0);

  // Seeds the editor once, when everything has settled (success or the expected
  // 404), so a later refetch can't overwrite edits in progress.
  useEffect(() => {
    if (initialized || !data.seed) return;
    setValues(data.seed.values);
    setDefinitionText(data.seed.definitionText);
    assets.seed(data.seed);
    setSaved({ definitionText: data.seed.definitionText, values: data.seed.values });
    setInitialized(true);
  }, [initialized, data.seed, assets.seed]);

  // Not an edit to save: the draft never shows a dropped id, and the next save or publish leaves it out.
  const dropFromSelection = useCallback((field: 'docIds' | 'subscriptionPlanIds', isUsable: (id: string) => boolean) => {
    const without = withoutUnusableIds(field, isUsable);
    setValues(without);
    setSaved((current) => ({ ...current, values: without(current.values) }));
  }, []);

  // The draft offers active plans only, so a plan deleted or deactivated elsewhere is dropped quietly
  // rather than submitted (the server refuses an inactive one).
  useEffect(() => {
    // Not while it is being read again: an older list may predate a plan activated since.
    if (!initialized || !plans.plans || !plans.upToDate) return;
    const active = new Set(plans.plans.filter((plan) => plan.status === 'ACTIVE').map((plan) => plan.id));
    const isActive = (id: string) => active.has(id);
    if (values.subscriptionPlanIds.every(isActive)) return;
    dropFromSelection('subscriptionPlanIds', isActive);
  }, [dropFromSelection, initialized, plans.plans, plans.upToDate, values.subscriptionPlanIds]);

  // A document deleted elsewhere is not listed but would still be submitted, so drop it quietly.
  // Not while it is being read again: an older list may predate a document added since.
  useEffect(() => {
    if (!initialized || !documents.documents || !documents.upToDate) return;
    const known = new Set(documents.documents.map((doc) => doc.id));
    const isKnown = (id: string) => known.has(id);
    if (values.docIds.every(isKnown)) return;
    dropFromSelection('docIds', isKnown);
  }, [dropFromSelection, initialized, documents.documents, documents.upToDate, values.docIds]);

  /** What to tell the user about a failed call; a vanished draft gets a clearer message than the server's. */
  const describeFailure = (error: unknown, fallback: string): string =>
    getErrorCode(error) === 'DRAFT_NOT_FOUND' ? intl.formatMessage(messages.draftNotFound) : getErrorMessage(error, fallback);

  if (data.isLoading || (!initialized && !data.error)) {
    return (
      <AppPage>
        <LoadingState label={intl.formatMessage(messages.loading)} />
      </AppPage>
    );
  }

  if (data.error || !data.server) {
    return (
      <AppPage>
        {data.error ? (
          <ErrorAlert error={data.error} onRetry={data.refetch} />
        ) : (
          <ErrorState message={intl.formatMessage(messages.loadFailed)} />
        )}
      </AppPage>
    );
  }

  const { server } = data;
  const isPublished = Boolean(publication);
  const canDeprecate = publication?.status === 'PUBLISHED';
  const canManage = hasPermission(SCOPES.MCP_PROXY_API_PORTAL_MANAGE);

  // Published is off the switch while there is nothing live, so an unpublish that lands
  // while the published version is on screen falls back to the draft.
  const pick = chosenView === 'published' && !isPublished ? undefined : chosenView;
  const view: PublicationVersionTone = pick ?? (isPublished ? 'published' : 'draft');
  const showingPublished = view === 'published';

  const hasUnsavedChanges =
    definitionText !== saved.definitionText ||
    assets.hasUnsavedChanges ||
    (Object.keys(values) as (keyof DraftFormValues)[]).some((field) => {
      const current = values[field];
      const savedValue = saved.values[field];
      return Array.isArray(current) && Array.isArray(savedValue) ? !sameSelection(current, savedValue) : current !== savedValue;
    });

  const errors = validateDraftFormValues(values);
  const errorFor = (field: DraftFormField) => {
    const error = touched[field] ? errors[field] : undefined;
    return error && intl.formatMessage(error);
  };
  const formInvalid = Object.keys(errors).length > 0;
  const markTouched = (field: DraftFormField) => setTouched((current) => ({ ...current, [field]: true }));
  const touchAllFields = () => setTouched(Object.fromEntries(FORM_FIELDS.map((field) => [field, true])));

  /** An edited field's message waits for its next blur, so it never flashes while the user is still typing. */
  const handleChange = (next: DraftFormValues) => {
    const edited = FORM_FIELDS.filter((field) => next[field] !== values[field]);
    setTouched((current) => ({ ...current, ...Object.fromEntries(edited.map((field) => [field, false])) }));
    setValues(next);
  };

  /** False, sending nothing, when the form is invalid. */
  const saveDetails = async (): Promise<boolean> => {
    if (formInvalid) {
      touchAllFields();
      setTab('details');
      return false;
    }
    const stored = await mcpProxiesApis.saveMcpProxyApiPortalDraft(
      apiPortalId,
      serverId,
      draftFormValuesToInput(values, draft ?? publication),
      PLATFORM_API_BASE_URL
    );
    data.setDraft(stored);
    setSaved((current) => ({ ...current, values }));
    return true;
  };

  /** Whether the definition is complete is checked at publish. */
  const saveDefinition = async () => {
    await mcpProxiesApis.saveMcpProxyApiPortalDraftDefinition(
      apiPortalId,
      serverId,
      parseDefinitionText(definitionText),
      PLATFORM_API_BASE_URL
    );
    setSaved((current) => ({ ...current, definitionText }));
  };

  /**
   * Only a failure reads the page again, to show where things stand: a save keeps the draft it
   * stored, and the other actions leave the page.
   */
  const runAction = async (action: PendingAction, failure: string, perform: () => Promise<void>) => {
    setPendingAction(action);
    try {
      await perform();
    } catch (error) {
      showSnackbar(describeFailure(error, failure), 'error');
      data.refetch();
    } finally {
      setPendingAction('idle');
    }
  };

  const pickThumbnail = (file: File) => {
    const problem = validateThumbnailFile(file);
    if (problem) {
      showSnackbar(intl.formatMessage(problem), 'error');
      return;
    }
    assets.setThumbnail(file);
  };

  const handleSaveDraft = () =>
    runAction('saving', intl.formatMessage(messages.saveFailed), async () => {
      if (!(await saveDetails())) return;
      await saveDefinition();
      await assets.saveAssets();
      showSnackbar(intl.formatMessage(messages.draftSaved), 'success');
    });

  // Each terminal action returns to the hub list, whose card shows the new status.
  const backToHubsList = () => navigate(hubsPath);

  const handlePublish = () =>
    runAction('publishing', intl.formatMessage(messages.publishFailed), async () => {
      if (!(await saveDetails())) return;
      await saveDefinition();
      await assets.saveAssets();
      try {
        await mcpProxiesApis.publishSavedMcpProxyDraft(apiPortalId, serverId, PLATFORM_API_BASE_URL);
      } catch (error) {
        // runAction reports the message.
        if (getErrorCode(error) === 'PUBLICATION_VALIDATION_FAILED' && getFieldErrors(error)?.some((e) => e.field === 'definition')) {
          setTab('capabilities');
        }
        throw error;
      }
      showSnackbar(intl.formatMessage(messages.published, { hub: hubName }), 'success');
      backToHubsList();
    });

  const confirmUnpublish = () => {
    setConfirmingUnpublish(false);
    return runAction('unpublishing', intl.formatMessage(messages.unpublishFailed), async () => {
      await mcpProxiesApis.unpublishMcpProxyFromApiPortal(apiPortalId, serverId, PLATFORM_API_BASE_URL);
      showSnackbar(intl.formatMessage(messages.unpublished, { hub: hubName }), 'success');
      backToHubsList();
    });
  };

  const confirmDeprecate = () => {
    setConfirmingDeprecate(false);
    return runAction('deprecating', intl.formatMessage(messages.deprecateFailed), async () => {
      await mcpProxiesApis.deprecateMcpProxyOnApiPortal(apiPortalId, serverId, PLATFORM_API_BASE_URL);
      showSnackbar(intl.formatMessage(messages.deprecated, { hub: hubName }), 'success');
      backToHubsList();
    });
  };

  const publishedValues = resolveDraftFormValues(undefined, publication, undefined);
  const savedPlanIds = resolveDraftFormValues(draft, publication, undefined).subscriptionPlanIds;

  // Only when it last changed: the switch already says which version this is, and the form shows its number.
  const draftBannerMeta = (): ReactNode => {
    if (!hasUnsavedChanges) {
      return draft && intl.formatMessage(messages.edited, { time: formatRelativeTime(draft.updatedAt) });
    }
    return draft ? (
      <FormattedMessage
        {...messages.editedUnsaved}
        values={{ time: formatRelativeTime(draft.updatedAt), unsaved: unsavedChanges }}
      />
    ) : (
      unsavedChanges
    );
  };
  const draftMeta = draftBannerMeta();

  const banner = showingPublished
    ? {
        meta: canDeprecate
          ? intl.formatMessage(messages.updated, { time: formatRelativeTime(publication?.updatedAt) })
          : intl.formatMessage(messages.deprecatedAt, { time: formatRelativeTime(publication?.updatedAt) }),
      }
    : draftMeta
      ? { meta: draftMeta }
      : undefined;

  const renderContent = () => {
    if (showingPublished) {
      if (tab === 'subscriptionPlans') {
        return (
          <SubscriptionPlansTab
            deletedPlans={deletedPlansOf(publication)}
            editedPlanIds={editedPlanIdsOf(publication)}
            failed={plans.failed}
            plans={plans.plans}
            readOnly
            storedNames={planNamesOf(publication)}
            values={publishedValues}
          />
        );
      }
      if (tab === 'documentations') {
        return (
          <DocumentsTab
            deletedDocuments={deletedDocumentsOf(publication)}
            documents={documents.documents}
            editedDocIds={editedDocIdsOf(publication)}
            failed={documents.failed}
            mcpProxyId={serverId}
            readOnly
            storedNames={documentNamesOf(publication)}
            values={publishedValues}
          />
        );
      }
      if (tab === 'landingPage') {
        return <LandingPageTab failed={data.publishedLandingPage.failed} readOnly text={data.publishedLandingPage.text} />;
      }
      if (tab === 'details') return <McpProxyDetailsTab readOnly thumbnailUrl={publishedThumbnailUrl} values={publishedValues} />;
      if (data.publishedDefinition.isLoading) {
        return <LoadingState label={intl.formatMessage(messages.loadingPublishedSpec)} />;
      }
      if (data.publishedDefinition.failed) {
        return <ErrorState message={intl.formatMessage(messages.publishedSpecFailed)} />;
      }
      return <CapabilitiesTab readOnly text={definitionToText(data.publishedDefinition.definition)} />;
    }

    if (tab === 'subscriptionPlans') {
      return (
        <SubscriptionPlansTab
          disabled={busy}
          failed={plans.failed}
          onChange={handleChange}
          plans={plans.plans}
          savedPlanIds={savedPlanIds}
          values={values}
        />
      );
    }
    if (tab === 'landingPage') {
      return (
        <LandingPageTab
          disabled={busy}
          failed={assets.landingPageFailed}
          generated={assets.generatedLandingPage}
          onChange={assets.setLandingPage}
          onCreate={() => assets.createLandingPage(buildLandingPageStarter(values, definitionText))}
          text={assets.landingPage}
        />
      );
    }
    if (tab === 'documentations') {
      return (
        <DocumentsTab
          disabled={busy}
          documents={documents.documents}
          failed={documents.failed}
          mcpProxyId={serverId}
          onChange={handleChange}
          values={values}
        />
      );
    }
    if (tab === 'details') {
      return (
        <McpProxyDetailsTab
          disabled={busy}
          errors={{
            displayName: errorFor('displayName'),
            version: errorFor('version'),
            productionUrl: errorFor('productionUrl'),
            sandboxUrl: errorFor('sandboxUrl'),
          }}
          onBlurField={markTouched}
          onChange={handleChange}
          onThumbnailPick={pickThumbnail}
          productionUrlOptions={data.productionUrlOptions}
          thumbnailUrl={thumbnailUrl}
          values={values}
        />
      );
    }
    return (
      <CapabilitiesTab
        available={server.capabilities as CapabilitiesTabProps['available']}
        disabled={busy}
        onChange={setDefinitionText}
        text={definitionText}
      />
    );
  };

  return (
    // Fills the screen, so the bottom gap is the control plane's rather than the default 64px that would shrink the card.
    <AppPage sx={{ pb: 5 }}>
      <Box ref={fill.ref} sx={{ display: 'flex', flexDirection: 'column', height: fill.height, minHeight: 0 }}>
        <Button
          component={Link}
          size="small"
          startIcon={<ChevronLeft size={24} />}
          sx={{ alignSelf: 'flex-start' }}
          to={hubsPath}
        >
          <FormattedMessage {...messages.back} />
        </Button>
        <PageTitle>
          <PageTitle.Header>
            <FormattedMessage {...messages.title} values={{ name: bold(server.displayName) }} />
          </PageTitle.Header>
          <PageTitle.SubHeader>
            <FormattedMessage {...messages.subtitle} values={{ hub: bold(hubName) }} />
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
              <Tab label={intl.formatMessage(messages.tabCapabilities)} value="capabilities" />
              <Tab label={intl.formatMessage(messages.tabPlans)} value="subscriptionPlans" />
              <Tab label={intl.formatMessage(messages.tabDocuments)} value="documentations" />
              <Tab label={intl.formatMessage(messages.tabLanding)} value="landingPage" />
            </Tabs>
            <PublicationVersionToggle
              disabled={busy}
              onChange={setChosenView}
              publishedAvailable={isPublished}
              value={view}
            />
          </Stack>
          {assets.unreadable && !showingPublished && (
            <Alert severity="warning" sx={{ flexShrink: 0 }}>
              <FormattedMessage {...messages.assetsUnreadable} />
            </Alert>
          )}
          <PublicationVersionCard banner={banner} tone={view}>
            {renderContent()}
          </PublicationVersionCard>
          {/* Reserves the bar's row while the published version is on screen, so the card doesn't resize. */}
          <Box sx={{ minHeight: ACTION_BAR_HEIGHT }}>
            {!showingPublished && (
              <PublishActionsBar
                canDeprecate={canDeprecate}
                canSaveDraft={hasUnsavedChanges}
                deprecating={pendingAction === 'deprecating'}
                isPublished={isPublished}
                onDeprecate={() => setConfirmingDeprecate(true)}
                onPublish={handlePublish}
                onSaveDraft={handleSaveDraft}
                onUnpublish={() => setConfirmingUnpublish(true)}
                permissions={{ publish: canManage, unpublish: canManage, deprecate: canManage }}
                publishing={pendingAction === 'publishing'}
                saveBlocked={assets.unreadable}
                savingDraft={pendingAction === 'saving'}
                unpublishing={pendingAction === 'unpublishing'}
              />
            )}
          </Box>
        </Stack>
      </Box>
      <ConfirmDialog
        confirmInputLabel={intl.formatMessage(messages.typeToConfirm, { name: server.displayName })}
        confirmPhrase={server.displayName}
        destructive
        loading={pendingAction === 'unpublishing'}
        message={intl.formatMessage(messages.unpublishMessage, { name: server.displayName, hub: hubName })}
        onCancel={() => setConfirmingUnpublish(false)}
        onConfirm={confirmUnpublish}
        open={confirmingUnpublish}
        title={intl.formatMessage(messages.unpublishTitle)}
      />
      <ConfirmDialog
        confirmColor="warning"
        confirmInputLabel={intl.formatMessage(messages.typeToConfirm, { name: server.displayName })}
        confirmPhrase={server.displayName}
        loading={pendingAction === 'deprecating'}
        message={intl.formatMessage(messages.deprecateMessage, { name: server.displayName, hub: hubName })}
        onCancel={() => setConfirmingDeprecate(false)}
        onConfirm={confirmDeprecate}
        open={confirmingDeprecate}
        title={intl.formatMessage(messages.deprecateTitle)}
      />
    </AppPage>
  );
}
