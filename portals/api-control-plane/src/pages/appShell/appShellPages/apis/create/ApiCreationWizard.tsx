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

import { Box, Button, LinearProgress, Stack, Typography } from '@wso2/oxygen-ui';
import { ArrowRight } from '@wso2/oxygen-ui-icons-react';
import { useCallback, useEffect, useRef, useState } from 'react';
import { defineMessages, useIntl } from 'react-intl';
import { useNavigate } from 'react-router-dom';
import { DefineApiPanel } from './components/DefineApiPanel';
import { GeneralCreateApiForm } from './components/GeneralCreateApiForm';
import { ApiCreationWizardDraftState, ApiType, GeneralApiCreationFormState } from './types';
import { ApiTypeSelector } from './components/ApiTypeSelector';
import type { ApiCreationStepKey } from './components/ApiCreationSteps';
import { AppPage } from '@/components/AppPage';
import { useNotifications } from '@/components/Notifications';
import { useImportOpenApi } from '@/api/resources/restApis';
import { useConsoleScope } from '@/scope/ConsoleScopeProvider';
import { routes } from '@/routes/paths';
import { toCreateApiFormErrors, type CreateApiFormErrors } from './utils/serverFieldErrors';
import {
  ApiCreationProgress,
  type ApiCreationProgressStatus,
} from './components/ApiCreationProgress';
import { API_TYPES } from './uiConfig';
import { ApiDesignerBanner } from './components/ApiDesignerBanner';
import { skeletonFor } from './utils/apiSkeleton';
import type { ApiError } from '@/api/core/errors';

const CONFIGURE_FORM_ID = 'api-creation-configure-form';

const messages = defineMessages({
  back: {
    id: 'api.create.ApiCreationWizard.action.back',
    defaultMessage: 'Back',
  },
  continue: {
    id: 'api.create.ApiCreationWizard.action.continue',
    defaultMessage: 'Continue',
  },
  apiTypeSubtitle: {
    id: 'api.create.ApiCreationWizard.apiType.subtitle',
    defaultMessage: 'Choose how the gateway should expose your backend.',
  },
  apiTypeTitle: {
    id: 'api.create.ApiCreationWizard.apiType.title',
    defaultMessage: 'What kind of API are you exposing?',
  },
  createdNextStep: {
    id: 'api.create.ApiCreationWizard.created.nextStep',
    defaultMessage: 'API created. Deploy it to a gateway to start serving traffic.',
  },
  createdNextStepAction: {
    id: 'api.create.ApiCreationWizard.created.nextStepAction',
    defaultMessage: 'Deploy',
    description: 'Button in the "API created" notification that opens the Deploy page.',
  },
  configureSubtitle: {
    id: 'api.create.ApiCreationWizard.configure.subtitle',
    defaultMessage: 'Review the API details, configure its backend endpoint, and create it.',
  },
  configureTitle: {
    id: 'api.create.ApiCreationWizard.configure.title',
    defaultMessage: 'Configure and create',
  },
  genericApiType: {
    id: 'api.create.ApiCreationWizard.apiType.generic',
    defaultMessage: 'API',
    description:
      'Stands in for the chosen API type when the wizard is opened straight at a later step.',
  },
  sourceSubtitle: {
    id: 'api.create.ApiCreationWizard.source.subtitle',
    defaultMessage: 'Point us at a running service, or bring an OpenAPI spec that describes it.',
  },
  sourceTitle: {
    id: 'api.create.ApiCreationWizard.source.title',
    defaultMessage: 'How do you want to define your {apiType}?',
    description:
      '{apiType} is the type picked in the first step, e.g. "REST API". Reads as one sentence.',
  },
  cancel: {
    id: 'api.create.ApiCreationWizard.action.cancel',
    defaultMessage: 'Cancel',
    description: 'Leaves the wizard without creating anything, back to the project’s APIs.',
  },
  stepCount: {
    id: 'api.create.ApiCreationWizard.stepCount',
    defaultMessage: 'Step {current} of 3',
  },
  specTooLarge: {
    id: 'api.create.ApiCreationWizard.error.specTooLarge',
    defaultMessage: 'This OpenAPI spec is larger than the maximum allowed size.',
  },
});

export const ApiCreationWizard = () => (
  <AppPage hideBreadcrumbs>
    <ApiCreationWizardContent />
  </AppPage>
);

const ApiCreationWizardContent = () => {
  const intl = useIntl();
  const [step, setStep] = useState<ApiCreationStepKey>('apiType');
  const [apiType, setApiType] = useState<ApiType | null>(
    () => API_TYPES.find((candidate) => candidate.enabled) ?? null,
  );
  const [sourceDraft, setSourceDraft] = useState<ApiCreationWizardDraftState | null>(null);
  /**
   * Steps stay mounted once visited and are only hidden, so Back and Next keep
   * what was entered on them rather than rebuilding each screen.
   */
  const [visited, setVisited] = useState<{ configure: boolean; source: boolean }>({
    configure: false,
    source: false,
  });
  /** The source draft the details step was last filled from. */
  const configuredFromRef = useRef<ApiCreationWizardDraftState | null>(null);
  /** Bumped when the details step must start over from a new source. */
  const [configureKey, setConfigureKey] = useState(0);

  /**
   * Each step's heading takes focus when the step changes, so keyboard and
   * screen-reader users land at the top of the new step rather than on the
   * page body, and the page scrolls back to where the step begins. Not on
   * first render: arriving at the page is the browser's to announce.
   */
  const headingRef = useRef<HTMLHeadingElement>(null);
  const shownStepRef = useRef(step);
  useEffect(() => {
    if (shownStepRef.current === step) return;
    shownStepRef.current = step;
    headingRef.current?.focus();
  }, [step]);

  const [prefilledData, setPrefilledData] = useState<Partial<GeneralApiCreationFormState>>({});
  /**
   * Why the last attempt was rejected, when the form is where it belongs.
   * Cleared on the next submission, not on the way back — the form is what
   * renders it, and it has to survive being returned to.
   */
  const [serverErrors, setServerErrors] = useState<CreateApiFormErrors | null>(null);
  const [identifierEdited, setIdentifierEdited] = useState(false);
  const [basePathEdited, setBasePathEdited] = useState(false);
  const [upstreamEdited, setUpstreamEdited] = useState(false);

  /**
   * The chosen type's own name, translated. `apiType` is already the entry from
   * the catalog, so its descriptor is read directly rather than looked up again.
   */
  const apiTypeName =
    apiType === null
      ? intl.formatMessage(messages.genericApiType)
      : intl.formatMessage(apiType.title);

  const getTitleForStep = (step: ApiCreationStepKey) => {
    switch (step) {
      case 'apiType':
        return intl.formatMessage(messages.apiTypeTitle);
      case 'source':
        return intl.formatMessage(messages.sourceTitle, { apiType: apiTypeName });
      case 'configure':
        return intl.formatMessage(messages.configureTitle);
      default:
        return '';
    }
  };

  const getSubtitleForStep = (step: ApiCreationStepKey) => {
    switch (step) {
      case 'apiType':
        return intl.formatMessage(messages.apiTypeSubtitle);
      case 'source':
        return intl.formatMessage(messages.sourceSubtitle);
      case 'configure':
        return intl.formatMessage(messages.configureSubtitle);
      default:
        return '';
    }
  };

  /**
   * Replaces the draft rather than merging into it: `extractApiDetails` omits
   * the keys a document doesn't answer for, so spreading over the previous
   * draft would carry an earlier import's fields into a later one.
   *
   * A fresh import also supersedes any earlier submission, so the form starts
   * from the new document rather than restoring values typed against the old.
   */
  const continueFromSource = () => {
    if (sourceDraft === null) return;
    // The same source as last time: the details step keeps the user's edits.
    // A different one changes what those edits were made against, so
    // everything after the source starts over.
    if (sourceDraft !== configuredFromRef.current) {
      configuredFromRef.current = sourceDraft;
      setPrefilledData(sourceDraft);
      setSubmittedValues(null);
      setServerErrors(null);
      setIdentifierEdited(false);
      setBasePathEdited(false);
      setUpstreamEdited(false);
      setConfigureKey((key) => key + 1);
    }
    setVisited((previous) => ({ ...previous, configure: true }));
    setStep('configure');
  };

  const navigate = useNavigate();
  const { notify } = useNotifications();
  // `handlesErrors`: a rejection this screen puts back on the form must not
  // also arrive as a snackbar that has faded by the time the user looks up.
  const importOpenApiMutation = useImportOpenApi({ handlesErrors: true });
  // `projectId` on the request body is the project handle from the route, not
  // something the form collects.
  const { activeScope, params } = useConsoleScope();

  /**
   * What the last attempt was submitted with. Deliberately *not* cleared when
   * the progress screen is dismissed: the form remounts on the way back, and
   * this is what it has to start from if the user's own edits are to survive a
   * failed create. `creationStarted` — not this — decides which screen shows.
   */
  const [submittedValues, setSubmittedValues] = useState<GeneralApiCreationFormState | null>(null);
  /** Whether the progress screen stands in for the form. */
  const [creationStarted, setCreationStarted] = useState(false);

  /**
   * Leaves the progress screen for the details form. The form is remounted
   * from `submittedValues`, so it starts from exactly what was sent: that is
   * how it tells a server complaint that still stands from one the user has
   * since edited away. The steps before it are left as they were.
   */
  const returnToForm = () => {
    setConfigureKey((key) => key + 1);
    setCreationStarted(false);
  };

  const createApi = (values: GeneralApiCreationFormState) => {
    // A fresh attempt supersedes the previous rejection, so nothing stale is
    // left pinned to an input the user has since corrected.
    setServerErrors(null);
    const projectId = activeScope.projectHandler;
    if (!projectId || !values.contractImport?.specFile) {
      // Nothing to create against — the wizard is mounted outside a project,
      // or the spec was never produced (contract source with no loaded spec).
      setCreationStarted(false);
      return;
    }

    // Both "from contract" and "start from scratch" carry a spec file in the
    // draft: contract passes the imported spec (URL-sourced contracts too —
    // the bytes /validate-openapi returned in `content` are packaged into a
    // File by DefineApiPanel), scratch passes the skeleton. Both submit via
    // import-openapi. The backend still accepts `url` for direct REST callers.
    const formData = new FormData();
    const mainUrl = values.upstream?.main?.url?.trim();
    const specFile = values.contractImport.fromSkeleton
      ? new File(
          [
            JSON.stringify(
              skeletonFor({
                description: values.description?.trim() || undefined,
                displayName: values.displayName.trim(),
                upstreamUrl: mainUrl,
                version: values.version.trim(),
              }),
              null,
              2,
            ),
          ],
          values.contractImport.specFile.name,
          { type: 'application/json' },
        )
      : values.contractImport.specFile;
    formData.append('file', specFile, specFile.name);
    formData.append('id', values.id.trim());
    formData.append('displayName', values.displayName.trim());
    formData.append('version', values.version.trim());
    // Normalize context to always have a leading slash.
    const context = `/${values.context.trim().replace(/^\/+/, '')}`;
    formData.append('context', context);
    formData.append('projectId', projectId);
    if (values.description?.trim()) {
      formData.append('description', values.description.trim());
    }
    if (mainUrl) {
      formData.append('upstream', JSON.stringify({ main: { url: mainUrl } }));
    }
    importOpenApiMutation.mutate(formData, {
      onError: (error) => {
        const apiError = error as ApiError;
        if (apiError.status === 413 || apiError.code === 'PAYLOAD_TOO_LARGE') {
          returnToForm();
          setServerErrors({
            fields: {},
            message: intl.formatMessage(messages.specTooLarge),
            unmapped: [],
          });
          return;
        }
        const formErrors = toCreateApiFormErrors(apiError);
        if (formErrors) {
          returnToForm();
          setServerErrors(formErrors);
        }
      },
    });
  };

  // The spec was already validated (server-side) at pick time in ContractSourceForm,
  // so Create submits straight to import-openapi. If the spec is somehow invalid
  // by the time it reaches the backend, the create response carries the reason.
  const onGeneralFormSumit = async (finalData: GeneralApiCreationFormState) => {
    if (!activeScope.projectHandler) return;
    if (step !== 'configure') return;
    setSubmittedValues(finalData);
    setCreationStarted(true);
    createApi(finalData);
  };

  const activeMutation = importOpenApiMutation;

  /**
   * Redirect to the created API's overview page.
   * `replace` keeps Back from returning to a finished progress screen.
   */
  const goToCreatedApi = useCallback(() => {
    const { orgHandle, projectHandler } = params;
    if (!orgHandle || !projectHandler) return;

    const createdId = activeMutation.data?.id;
    navigate(
      createdId
        ? routes.api(orgHandle, projectHandler, createdId)
        : // Created, but the response carried no handle to navigate to.
          routes.apis(orgHandle, projectHandler),
      { replace: true },
    );
    if (createdId) {
      // Hand off to the next activation step rather than leaving the user to
      // find Deploy on their own.
      notify(intl.formatMessage(messages.createdNextStep), 'success', {
        label: intl.formatMessage(messages.createdNextStepAction),
        onClick: () => navigate(routes.apiDeploy(orgHandle, projectHandler, createdId)),
      });
    }
  }, [activeMutation.data?.id, intl, navigate, notify, params]);

  const creationStatus: ApiCreationProgressStatus = activeMutation.isError
    ? 'failed'
    : activeMutation.isSuccess
      ? 'created'
      : 'creating';

  // The progress screen stands in front of the wizard rather than replacing
  // it, so coming back from a failed create finds every step as it was left.
  const showProgress = creationStarted && submittedValues !== null;
  const progress = showProgress ? (
    <ApiCreationProgress
      displayName={submittedValues.displayName}
      onBack={() => {
        importOpenApiMutation.reset();
        // Only the screen goes back; `submittedValues` stays so the form
        // returns to what was typed rather than to the imported draft.
        returnToForm();
      }}
      onComplete={goToCreatedApi}
      onRetry={() => createApi(submittedValues)}
      status={creationStatus}
    />
  ) : null;

  const stepNumber = step === 'apiType' ? 1 : step === 'source' ? 2 : 3;

  return (
    <Stack spacing={2} sx={{ width: '100%' }}>
      {progress}
      <Box
        sx={{
          border: 1,
          borderColor: 'divider',
          borderRadius: 1,
          display: showProgress ? 'none' : 'flex',
          flexDirection: 'column',
          minHeight: 620,
          // `clip`, not `hidden`: it still rounds the corners, but doesn't make
          // the card a scroll container, which is what stopped the footer
          // below from sticking.
          overflow: 'clip',
          width: '100%',
        }}
      >
        <LinearProgress
          aria-label={intl.formatMessage(messages.stepCount, { current: stepNumber })}
          sx={{
            bgcolor: 'divider',
            height: 3,
            '& .MuiLinearProgress-bar': { bgcolor: 'primary.main' },
          }}
          value={(stepNumber / 3) * 100}
          variant="determinate"
        />

        <Box sx={{ flex: 1, p: { md: 3.5, xs: 2 } }}>
          <Stack
            direction="column"
            spacing={step === 'configure' ? 2 : 3}
            sx={{ alignItems: 'flex-start' }}
          >
            <Box>
              <Typography
                ref={headingRef}
                sx={{ fontWeight: 700, outline: 'none', textAlign: 'left' }}
                tabIndex={-1}
                variant="h1"
              >
                {getTitleForStep(step)}
              </Typography>
              <Typography variant="body1" sx={{ opacity: 0.65, textAlign: 'left' }}>
                {getSubtitleForStep(step)}
              </Typography>
            </Box>

            <Box sx={{ width: '100%' }}>
              {step === 'apiType' && (
                <ApiTypeSelector
                  onChange={(apiType) => {
                    setApiType(apiType);
                  }}
                  value={apiType?.key}
                />
              )}

              {visited.source && (
                <Box sx={{ display: step === 'source' ? 'block' : 'none' }}>
                  {/* Kept mounted once visited, so Back preserves the selected
                    source and its edits. A different API type is a different
                    source, so it starts the panel over. */}
                  <DefineApiPanel
                    initialApiTypeKey={apiType?.key}
                    key={apiType?.key ?? 'none'}
                    onDraftChange={setSourceDraft}
                  />
                </Box>
              )}

              {visited.configure && (
                <Box
                  sx={{
                    display: step === 'configure' ? 'block' : 'none',
                    maxWidth: { md: '80%', xs: '100%' },
                  }}
                >
                  <GeneralCreateApiForm
                    key={configureKey}
                    formId={CONFIGURE_FORM_ID}
                    hideActions
                    // What the user actually submitted, when there is such an
                    // attempt to come back from: the form remounts after the
                    // progress screen, so anything hand-typed would otherwise
                    // revert to the spec-derived draft.
                    initialValues={submittedValues ?? prefilledData}
                    onSubmit={onGeneralFormSumit}
                    onBack={() => setStep('source')}
                    serverErrors={serverErrors ?? undefined}
                    initialIdentifierEdited={identifierEdited}
                    onIdentifierEdited={setIdentifierEdited}
                    initialBasePathEdited={basePathEdited}
                    onBasePathEdited={setBasePathEdited}
                    initialUpstreamEdited={upstreamEdited}
                    onUpstreamEdited={() => setUpstreamEdited(true)}
                  />
                </Box>
              )}
            </Box>
          </Stack>
        </Box>

        {/* Back and Continue stay in view: the footer sticks to the bottom of
            the window while a long step scrolls beneath it. */}
        <Stack
          sx={{
            bgcolor: 'background.paper',
            borderColor: 'divider',
            borderTop: 1,
            bottom: 0,
            position: 'sticky',
            zIndex: 1,
          }}
        >
          <Stack
            direction="row"
            sx={{
              alignItems: 'center',
              justifyContent: 'space-between',
              minHeight: 56,
              px: 3,
            }}
          >
            <Stack alignItems="center" direction="row" spacing={2}>
              <Typography color="text.secondary" sx={{ fontWeight: 600 }} variant="caption">
                {intl.formatMessage(messages.stepCount, { current: stepNumber })}
              </Typography>
              {/* The wizard hides breadcrumbs and has no other exit, so
                leaving used to mean the browser's Back or the sidebar. */}
              {params.orgHandle && params.projectHandler && (
                <Button
                  color="inherit"
                  onClick={() => navigate(routes.apis(params.orgHandle!, params.projectHandler!))}
                  size="small"
                  type="button"
                  variant="text"
                >
                  {intl.formatMessage(messages.cancel)}
                </Button>
              )}
            </Stack>
            <Stack direction="row" spacing={1}>
              <Button
                disabled={step === 'apiType'}
                onClick={() => setStep(step === 'configure' ? 'source' : 'apiType')}
                type="button"
                variant="text"
              >
                {intl.formatMessage(messages.back)}
              </Button>
              {step === 'configure' ? (
                <Button form={CONFIGURE_FORM_ID} key="create-api" type="submit" variant="contained">
                  {intl.formatMessage({
                    id: 'api.create.generalForm.action.create',
                    defaultMessage: 'Create',
                  })}
                </Button>
              ) : (
                <Button
                  disabled={step === 'apiType' ? !apiType : sourceDraft === null}
                  endIcon={<ArrowRight size={16} />}
                  key="continue-wizard"
                  onClick={() => {
                    if (step === 'apiType') {
                      setVisited((previous) => ({ ...previous, source: true }));
                      setStep('source');
                    } else continueFromSource();
                  }}
                  type="button"
                  variant="contained"
                >
                  {intl.formatMessage(messages.continue)}
                </Button>
              )}
            </Stack>
          </Stack>
        </Stack>
      </Box>
      {step === 'apiType' && !showProgress && <ApiDesignerBanner />}
    </Stack>
  );
};
