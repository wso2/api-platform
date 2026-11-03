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

import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react';
import { alpha, Box, PageTitle, Stack, Tab, Tabs } from '@wso2/oxygen-ui';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';
import { Link, useLocation, useNavigate, useParams } from 'react-router-dom';

import {
  useDeprecateApiOnApiPortal,
  usePublishApiToApiPortal,
  useSaveApiPublicationDraft,
  useSaveApiPublicationDraftDefinition,
  useSaveApiPublicationDraftLandingPage,
  useSaveApiPublicationDraftThumbnail,
  useUnpublishApiFromApiPortal,
  type DraftDefinitionBody,
} from '@/api/resources/apiPublications';
import { isApiError, isErrorCode } from '@/api/core/errors';
import { AppPage } from '@/components/AppPage';
import { ConfirmDialog } from '@/components/ConfirmDialog';
import { useFillScrollArea } from '@/hooks/useFillScrollArea';
import { useFrozenWhile } from '@/hooks/useFrozenWhile';
import { useObjectUrl } from '@/api/core/objectUrl';
import { useFormatters } from '@/i18n/useFormatters';
import { useNotifications } from '@/components/Notifications';
import { useAllApiDocuments } from '@/api/resources/apiDocuments';
import { useAllSubscriptionPlans } from '@/api/resources/subscriptionPlans';
import { LoadingState } from '@/components/StateViews';
import { useConsoleScope } from '@/scope/ConsoleScopeProvider';
import { parseGraphQLSdl } from '../apis/create/utils/graphqlSchema';
import { parseSpecText } from '../apis/create/utils/specText';
import { ApiDetailsTab } from './components/ApiDetailsTab';
import { DocumentsTab } from './components/DocumentsTab';
import { LandingPageTab } from './components/LandingPageTab';
import { PublicationLoadError } from './components/PublicationLoadError';
import { PublicationVersionCard, type PublicationVersionTone } from './components/PublicationVersionCard';
import { PublicationVersionToggle } from './components/PublicationVersionToggle';
import { PublishActionsBar } from './components/PublishActionsBar';
import { PublishedSpecificationTab } from './components/PublishedSpecificationTab';
import { SpecificationTab } from './components/SpecificationTab';
import { SubscriptionPlansTab } from './components/SubscriptionPlansTab';
import { restPublishTarget, type PublishTarget } from './publishTarget';
import { usePublishPageData } from './usePublishPageData';
import { buildLandingPageStarter } from './utils/landingPageStarter';
import type { DefinitionFormat } from './utils/storedDefinition';
import {
  deletedDocumentsOf,
  deletedPlansOf,
  documentNamesOf,
  draftFormValuesToInput,
  editedDocIdsOf,
  editedPlanIdsOf,
  emptyDraftFormValues,
  planNamesOf,
  resolveDraftFormValues,
  sameDraftFormValues,
  validateDraftFormValues,
  validateThumbnailFile,
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
    defaultMessage: 'Publish <name>{apiName}</name>',
    description:
      'Page heading. {apiName} is the API display name, user-supplied; do not translate it. <name> sets it in bold.',
  },
  subtitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.PortalPublishPage.subtitle',
    defaultMessage: 'To <name>{portalName}</name>',
    description:
      'Byline under the heading, naming the portal being published to. {portalName} is the API Portal display name, user-supplied; do not translate it. <name> sets it in bold.',
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
  draftBannerMeta: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.PortalPublishPage.draftBannerMeta',
    defaultMessage: 'Edited {time}',
    description: 'Banner byline. {time} is a relative time such as "5 minutes ago".',
  },
  draftBannerMetaUnsaved: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.PortalPublishPage.draftBannerMetaUnsaved',
    defaultMessage: 'Edited {time} · <unsaved>Unsaved changes</unsaved>',
    description:
      'Banner byline when the fields on screen differ from the saved draft. {time} is a relative time such as "5 minutes ago". <unsaved> sets the warning in colour.',
  },
  unsavedBannerMeta: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.PortalPublishPage.unsavedBannerMeta',
    defaultMessage: '<unsaved>Unsaved changes</unsaved>',
    description:
      'Banner byline when the fields on screen have been edited but no draft has been saved yet. <unsaved> sets the warning in colour.',
  },
  publishedBannerMeta: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.PortalPublishPage.publishedBannerMeta',
    defaultMessage: 'Updated {time}',
    description: 'Banner byline. {time} is a relative time such as "2 days ago".',
  },
  deprecatedBannerMeta: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.PortalPublishPage.deprecatedBannerMeta',
    defaultMessage: 'Deprecated {time}',
    description: 'Banner byline when the live listing is flagged as deprecated. {time} is a relative time such as "2 days ago".',
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

/** Tinted highlight on the text; an inline span's padding does not affect line layout, so it can't grow the banner. */
const unsaved = (text: ReactNode) => (
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
    {text}
  </Box>
);

const bold = (name: ReactNode) => (
  <Box component="span" sx={{ fontWeight: 700 }}>
    {name}
  </Box>
);

/**
 * Whether a failed publish is about the definition: only when the server names it. Every other
 * validation failure (thumbnail, documents, size) names no field, and must not move the user.
 */
const definitionAtFault = (error: unknown): boolean =>
  isApiError(error) && error.fieldErrors.some((fieldError) => fieldError.field === 'definition');

/**
 * A failed API call is already reported by the global mutation snackbar, so the
 * action handlers only need to stop it from surfacing again as an unhandled
 * rejection. Anything that is not an `ApiError` is a real bug and still throws.
 */
const rethrowUnreported = (error: unknown): void => {
  if (!isApiError(error)) throw error;
};

/** Takes out of one selection the ids the draft cannot offer: ones the server no longer knows, or plans that are not active. */
const withoutUnusableIds =
  (field: 'docIds' | 'subscriptionPlanIds', isUsable: (id: string) => boolean) =>
  (current: DraftFormValues): DraftFormValues => ({ ...current, [field]: current[field].filter(isUsable) });

/** The fields that show a validation message. */
const FORM_FIELDS: DraftFormField[] = ['displayName', 'version', 'productionUrl', 'sandboxUrl'];

type PublishTab = 'details' | 'specification' | 'subscriptionPlans' | 'documentations' | 'landingPage';

/** Which action is currently in flight, so the right button (and only that one) shows busy. */
type PendingAction = 'idle' | 'saving' | 'publishing' | 'unpublishing' | 'deprecating';

/**
 * The publish/unpublish/deprecate flow for one API on one API Portal. Publish takes no body and
 * publishes what the draft already holds, so every save must succeed first.
 * No `ScopeGate`: only reachable from the Portals listing's card, which is already API-scoped.
 */
export function PortalPublishPage() {
  return (
    // Less room under the buttons than the standard page, so the tab area gets the height.
    <AppPage sx={{ pb: 2 }}>
      <RestPortalPublish />
    </AppPage>
  );
}

/** A REST API in console scope, published through the shared page body. */
function RestPortalPublish() {
  const { params } = useConsoleScope();
  const orgHandle = params.orgHandle ?? '';
  const projectHandler = params.projectHandler ?? '';
  const apiHandler = params.apiHandler ?? '';
  const target = useMemo(
    () => restPublishTarget(orgHandle, projectHandler, apiHandler),
    [apiHandler, orgHandle, projectHandler],
  );
  return <PortalPublishPageContent target={target} />;
}

/**
 * The page body for one API on one portal, shared by every API kind: each
 * supplies a `PublishTarget` naming the API, its type and where it links.
 */
export function PortalPublishPageContent({ target }: { target: PublishTarget }) {
  const { apiPortalId = '' } = useParams();
  const location = useLocation();
  const navigate = useNavigate();
  const intl = useIntl();
  const { relativeTime } = useFormatters();
  // The page fills the visible area and only its middle scrolls, so switching tabs
  // or opening the editor never moves the header or the action buttons.
  const fill = useFillScrollArea<HTMLDivElement>(MIN_PAGE_HEIGHT);
  const { notify } = useNotifications();
  const { apiHandle: apiHandler, apiType } = target;
  const isGraphQL = target.definitionKind === 'graphql';
  const defaultFormat: DefinitionFormat = isGraphQL ? 'graphql' : 'json';

  // Passed by the portal card that linked here; falls back to the handle on a
  // direct link or refresh, where there is no navigation state.
  const portalName = (location.state as { portalName?: string } | null)?.portalName ?? apiPortalId;

  const [tab, setTab] = useState<PublishTab>('details');
  // Which version the user picked; until they pick, the live version if there is one, else the draft.
  const [chosenView, setChosenView] = useState<PublicationVersionTone>();
  const data = usePublishPageData(apiPortalId, target, chosenView !== 'draft' && tab === 'specification');

  const plansQuery = useAllSubscriptionPlans();

  const saveDraftMutation = useSaveApiPublicationDraft();
  const saveDefinitionMutation = useSaveApiPublicationDraftDefinition({ handlesErrors: true });
  const saveThumbnailMutation = useSaveApiPublicationDraftThumbnail({ handlesErrors: true });
  const saveLandingPageMutation = useSaveApiPublicationDraftLandingPage({ handlesErrors: true });
  const publishMutation = usePublishApiToApiPortal();
  const unpublishMutation = useUnpublishApiFromApiPortal();
  const deprecateMutation = useDeprecateApiOnApiPortal();

  const [values, setValues] = useState<DraftFormValues>(emptyDraftFormValues);
  const [touched, setTouched] = useState<Partial<Record<DraftFormField, boolean>>>({});
  const [definitionText, setDefinitionText] = useState('');
  const [definitionFormat, setDefinitionFormat] = useState<DefinitionFormat>(defaultFormat);
  const [definitionParseError, setDefinitionParseError] = useState<string>();
  // The thumbnail shown, and the draft's own; saving sends it only when they differ.
  const [thumbnail, setThumbnail] = useState<Blob>();
  const [storedThumbnail, setStoredThumbnail] = useState<Blob>();
  // The landing page's Markdown, and the draft's own; `undefined` until one exists.
  const [landingPage, setLandingPage] = useState<string>();
  const [storedLandingPage, setStoredLandingPage] = useState<string>();
  // The generated starter, to tell it apart from a page edited since.
  const [generatedLandingPage, setGeneratedLandingPage] = useState<string>();
  // Set once when the page opens, so a later refetch can't turn a loaded page into an error.
  const [landingPageFailed, setLandingPageFailed] = useState(false);
  // What the server holds for the fields shown, so edits made since can be told apart.
  const [saved, setSaved] = useState<{
    definitionText: string;
    landingPage?: string;
    thumbnail?: Blob;
    values: DraftFormValues;
  }>({
    definitionText: '',
    values: emptyDraftFormValues,
  });
  const [pendingAction, setPendingAction] = useState<PendingAction>('idle');
  const busy = pendingAction !== 'idle';
  // An action saves in steps, each refetching these; they are shown once, when it ends.
  const draft = useFrozenWhile(data.draft, busy);
  const publication = useFrozenWhile(data.publication, busy);
  const [confirmingUnpublish, setConfirmingUnpublish] = useState(false);
  const [confirmingDeprecate, setConfirmingDeprecate] = useState(false);
  const [initialized, setInitialized] = useState(false);

  // Seeds the editor once, when every tier has settled (success or the expected
  // 404), so a later refetch can't overwrite edits in progress.
  useEffect(() => {
    if (initialized || !data.seed) return;
    setValues(data.seed.values);
    setDefinitionText(data.seed.definition?.text ?? '');
    setThumbnail(data.seed.thumbnail?.blob);
    setStoredThumbnail(data.seed.thumbnail?.inDraft ? data.seed.thumbnail.blob : undefined);
    setLandingPage(data.seed.landingPage?.text);
    setStoredLandingPage(data.seed.landingPage?.inDraft ? data.seed.landingPage.text : undefined);
    setLandingPageFailed(data.seed.landingPageFailed);
    setSaved({
      definitionText: data.seed.definition?.text ?? '',
      landingPage: data.seed.landingPage?.text,
      thumbnail: data.seed.thumbnail?.blob,
      values: data.seed.values,
    });
    setDefinitionFormat(data.seed.definition?.format ?? defaultFormat);
    setInitialized(true);
  }, [initialized, data.seed, defaultFormat]);

  // Not an edit to save: the draft never shows a dropped id, and the next save or publish leaves it out.
  const dropFromSelection = useCallback((field: 'docIds' | 'subscriptionPlanIds', isUsable: (id: string) => boolean) => {
    const without = withoutUnusableIds(field, isUsable);
    setValues(without);
    setSaved((current) => ({ ...current, values: without(current.values) }));
  }, []);

  // The draft offers active plans only, so a plan deleted or deactivated elsewhere is dropped quietly
  // rather than submitted (the server refuses an inactive one).
  useEffect(() => {
    // Not while it is being read again: an older cached list may predate a plan activated since.
    if (!initialized || !plansQuery.isSuccess || plansQuery.isFetching) return;
    const active = new Set(plansQuery.data.list.filter((plan) => plan.status === 'ACTIVE').map((plan) => plan.id));
    const isActive = (id: string) => active.has(id);
    if (values.subscriptionPlanIds.every(isActive)) return;
    dropFromSelection('subscriptionPlanIds', isActive);
  }, [dropFromSelection, initialized, plansQuery.data, plansQuery.isFetching, plansQuery.isSuccess, values.subscriptionPlanIds]);

  // A document deleted elsewhere is not listed but would still be submitted, so drop it quietly.
  // Read only while a selection is held; the Documentation tab shares this query.
  // Not while it is being read again: an older cached list may predate a document added since.
  const documentsQuery = useAllApiDocuments(apiType, values.docIds.length > 0 ? apiHandler : undefined);
  useEffect(() => {
    if (!initialized || !documentsQuery.isSuccess || documentsQuery.isFetching) return;
    // A list cut short can't tell a deleted document from one it never loaded.
    if (documentsQuery.data.list.length < documentsQuery.data.pagination.total) return;
    const known = new Set(documentsQuery.data.list.map((doc) => doc.id));
    const isKnown = (id: string) => known.has(id);
    if (values.docIds.every(isKnown)) return;
    dropFromSelection('docIds', isKnown);
  }, [dropFromSelection, initialized, documentsQuery.data, documentsQuery.isFetching, documentsQuery.isSuccess, values.docIds]);

  const thumbnailUrl = useObjectUrl(thumbnail);
  const publishedThumbnailUrl = useObjectUrl(data.publishedThumbnail);

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
  // Published is off the switch while there is nothing live, so an unpublish that lands
  // while the published version is on screen falls back to the draft.
  const pick = chosenView === 'published' && !isPublished ? undefined : chosenView;
  const view: PublicationVersionTone = pick ?? (isPublished ? 'published' : 'draft');
  const showingPublished = view === 'published';
  const hasUnsavedChanges =
    definitionText !== saved.definitionText ||
    thumbnail !== saved.thumbnail ||
    landingPage !== saved.landingPage ||
    !sameDraftFormValues(values, saved.values);
  const errors = validateDraftFormValues(values);
  const errorFor = (field: DraftFormField) => (touched[field] ? errors[field] : undefined);
  const formInvalid = Object.keys(errors).length > 0;

  const markTouched = (field: DraftFormField) =>
    setTouched((current) => ({ ...current, [field]: true }));

  const touchAllFields = () => setTouched(Object.fromEntries(FORM_FIELDS.map((field) => [field, true])));

  /** An edited field's message waits for its next blur, so it never flashes while the user is still typing. */
  const handleChange = (next: DraftFormValues) => {
    const edited = FORM_FIELDS.filter((field) => next[field] !== values[field]);
    setTouched((current) => ({ ...current, ...Object.fromEntries(edited.map((field) => [field, false])) }));
    setValues(next);
  };

  /** The definition as the server expects it. Only unparseable JSON/YAML (or SDL) blocks the save; completeness is checked at publish. */
  const parseDefinition = (): DraftDefinitionBody | undefined => {
    if (definitionFormat === 'graphql') {
      const sdlResult = definitionText.trim() === '' ? undefined : parseGraphQLSdl(definitionText);
      if (sdlResult && 'error' in sdlResult) {
        setDefinitionParseError(sdlResult.error);
        setTab('specification');
        return undefined;
      }
      setDefinitionParseError(undefined);
      return { sdl: definitionText };
    }
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

  /** A draft that is gone (published from another tab) gets a clearer message than the server's 404 text; the mutations opt out of the global snackbar, so other failures are reported here. */
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
      apiType,
      apiId: apiHandler,
      body: draftFormValuesToInput(values),
    });
    setSaved((current) => ({ ...current, values }));
    return true;
  };

  /** Saves the Specification definition alone. False, untouched, when the buffer doesn't parse. */
  const saveDefinition = async (): Promise<boolean> => {
    const definitionDocument = parseDefinition();
    if (!definitionDocument) return false;
    await reportingMissingDraft(
      saveDefinitionMutation.mutateAsync({
        apiPortalId,
        apiType,
        apiId: apiHandler,
        body: definitionDocument,
      }),
    );
    setSaved((current) => ({ ...current, definitionText }));
    return true;
  };

  /** Saves the thumbnail unless the draft already stores it. A refused image reverts to the last saved one so it can't fail every later save; any other failure keeps it for retry. */
  const saveThumbnail = async () => {
    if (!thumbnail || thumbnail === storedThumbnail) return;
    try {
      await reportingMissingDraft(
        saveThumbnailMutation.mutateAsync({ apiPortalId, apiType, apiId: apiHandler, file: thumbnail }),
      );
    } catch (error) {
      if (isApiError(error) && (error.isValidation || error.status === 413)) setThumbnail(saved.thumbnail);
      throw error;
    }
    setStoredThumbnail(thumbnail);
    setSaved((current) => ({ ...current, thumbnail }));
  };

  /** Saves the landing page unless the draft already stores it. */
  const saveLandingPage = async () => {
    if (landingPage === undefined || landingPage === storedLandingPage) return;
    await reportingMissingDraft(
      saveLandingPageMutation.mutateAsync({ apiPortalId, apiType, apiId: apiHandler, markdown: landingPage }),
    );
    setStoredLandingPage(landingPage);
    setSaved((current) => ({ ...current, landingPage }));
  };

  /**
   * Saves the thumbnail and the landing page, each on its own: one failing must not leave the other unsaved.
   * The first failure is rethrown once both have been tried, so the caller still stops.
   */
  const saveAssets = async () => {
    const results = await Promise.allSettled([saveThumbnail(), saveLandingPage()]);
    const failed = results.find((result): result is PromiseRejectedResult => result.status === 'rejected');
    if (failed) throw failed.reason;
  };

  /** Starts the landing page from the details and the definition as they stand now. */
  const createLandingPage = () => {
    const starter = buildLandingPageStarter(
      values,
      definitionText.trim() ? { format: definitionFormat, text: definitionText } : undefined,
    );
    setLandingPage(starter);
    setGeneratedLandingPage(starter);
  };

  /** Takes an image the user picked as the thumbnail, unless the server would refuse it. */
  const pickThumbnail = (file: File) => {
    const problem = validateThumbnailFile(file);
    if (problem) {
      notify(intl.formatMessage(problem), 'error');
      return;
    }
    setThumbnail(file);
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
      await saveAssets();
      notify(
        intl.formatMessage(definitionSaved ? messages.draftSaved : messages.draftSavedDetailsOnly),
        definitionSaved ? 'success' : 'warning',
      );
    });

  // Each terminal action leaves this one portal's page behind for the listing,
  // where the card now reflects the new status — there's nothing left to do
  // on this page once the action the user came here for has gone through.
  const backToPortalsList = () => navigate(target.portalsPath);

  const handlePublish = () =>
    runAction('publishing', async () => {
      if (!(await saveDetails())) return;
      // Publish requires a saved, parseable definition — never proceed on a
      // partial save, which would publish whatever definition was already
      // there rather than what the user is looking at.
      if (!(await saveDefinition())) return;
      await saveAssets();
      try {
        await publishMutation.mutateAsync({ apiPortalId, apiType, apiId: apiHandler });
      } catch (error) {
        // The message is already reported by the global snackbar; this only sends the user to
        // the definition when it is the one at fault.
        if (isErrorCode(error, 'PUBLICATION_VALIDATION_FAILED') && definitionAtFault(error)) setTab('specification');
        throw error;
      }
      notify(intl.formatMessage(messages.published, { portalName }), 'success');
      backToPortalsList();
    });

  const confirmUnpublish = () => {
    setConfirmingUnpublish(false);
    return runAction('unpublishing', async () => {
      await unpublishMutation.mutateAsync({ apiPortalId, apiType, apiId: apiHandler });
      notify(intl.formatMessage(messages.unpublished, { portalName }), 'success');
      backToPortalsList();
    });
  };

  const confirmDeprecate = () => {
    setConfirmingDeprecate(false);
    return runAction('deprecating', async () => {
      await deprecateMutation.mutateAsync({ apiPortalId, apiType, apiId: apiHandler });
      notify(intl.formatMessage(messages.deprecated, { portalName }), 'success');
      backToPortalsList();
    });
  };

  const publishedValues = resolveDraftFormValues(undefined, publication, undefined);
  // Only when it last changed: the switch already says which version this is, and the form shows its number.
  const draftBannerMeta = (() => {
    if (!hasUnsavedChanges) return draft && intl.formatMessage(messages.draftBannerMeta, { time: relativeTime(draft.updatedAt) });
    return draft
      ? intl.formatMessage(messages.draftBannerMetaUnsaved, { time: relativeTime(draft.updatedAt), unsaved })
      : intl.formatMessage(messages.unsavedBannerMeta, { unsaved });
  })();
  const savedPlanIds = resolveDraftFormValues(draft, publication, undefined).subscriptionPlanIds;
  const publishedDeletedPlans = deletedPlansOf(publication);
  const publishedEditedPlanIds = editedPlanIdsOf(publication);
  const publishedDeletedDocuments = deletedDocumentsOf(publication);
  const publishedEditedDocIds = editedDocIdsOf(publication);
  const banner = showingPublished
    ? {
        meta: intl.formatMessage(canDeprecate ? messages.publishedBannerMeta : messages.deprecatedBannerMeta, {
          time: relativeTime(publication?.updatedAt),
        }),
      }
    : draftBannerMeta
      ? { meta: draftBannerMeta }
      : undefined;

  /** The live listing's tab, read-only and from what is already loaded. */
  const renderPublishedContent = () => {
    switch (tab) {
      case 'details':
        return <ApiDetailsTab readOnly thumbnailUrl={publishedThumbnailUrl} values={publishedValues} />;
      case 'subscriptionPlans':
        return (
          <SubscriptionPlansTab
            deletedPlans={publishedDeletedPlans}
            editedPlanIds={publishedEditedPlanIds}
            readOnly
            storedNames={planNamesOf(publication)}
            values={publishedValues}
          />
        );
      case 'documentations':
        return (
          <DocumentsTab
            apiHandle={apiHandler}
            apiType={apiType}
            deletedDocuments={publishedDeletedDocuments}
            editedDocIds={publishedEditedDocIds}
            readOnly
            storedNames={documentNamesOf(publication)}
            values={publishedValues}
          />
        );
      case 'landingPage':
        return (
          <LandingPageTab
            failed={data.publishedLandingPage.failed}
            isLoading={data.publishedLandingPage.isLoading}
            readOnly
            text={data.publishedLandingPage.text}
          />
        );
      case 'specification':
        return (
          <PublishedSpecificationTab
            definition={data.publishedDefinition.definition}
            failed={data.publishedDefinition.failed}
            isLoading={data.publishedDefinition.isLoading}
          />
        );
    }
  };

  /** The draft's tab, editable. */
  const renderDraftContent = () => {
    switch (tab) {
      case 'details':
        return (
          <ApiDetailsTab
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
      case 'subscriptionPlans':
        return (
          <SubscriptionPlansTab
            disabled={busy}
            onChange={handleChange}
            savedPlanIds={savedPlanIds}
            values={values}
          />
        );
      case 'documentations':
        return (
          <DocumentsTab
            apiHandle={apiHandler}
            apiType={apiType}
            disabled={busy}
            onChange={handleChange}
            values={values}
          />
        );
      case 'landingPage':
        return (
          <LandingPageTab
            disabled={busy}
            failed={landingPageFailed}
            generated={generatedLandingPage !== undefined && landingPage === generatedLandingPage}
            onChange={setLandingPage}
            onCreate={createLandingPage}
            text={landingPage}
          />
        );
      case 'specification':
        return (
          <SpecificationTab
            disabled={busy}
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
    }
  };

  return (
    <>
      <Box ref={fill.ref} sx={{ display: 'flex', flexDirection: 'column', height: fill.height, minHeight: 0 }}>
        <PageTitle sx={{ mb: 3 }}>
          <PageTitle.BackButton component={<Link to={target.portalsPath} />}>
            <FormattedMessage {...messages.back} />
          </PageTitle.BackButton>
          <PageTitle.Header>
            <FormattedMessage {...messages.title} values={{ apiName: api.displayName, name: bold }} />
          </PageTitle.Header>
          <PageTitle.SubHeader>
            <FormattedMessage {...messages.subtitle} values={{ name: bold, portalName }} />
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
              <Tab label={intl.formatMessage(messages.tabDocumentations)} value="documentations" />
              <Tab label={intl.formatMessage(messages.tabLandingPage)} value="landingPage" />
            </Tabs>
            <PublicationVersionToggle
              disabled={busy}
              onChange={setChosenView}
              publishedAvailable={isPublished}
              value={view}
            />
          </Stack>

          <PublicationVersionCard banner={banner} tone={view}>
            {showingPublished ? renderPublishedContent() : renderDraftContent()}
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
