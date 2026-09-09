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

import { Box, Button, Divider, Stack, Typography } from '@wso2/oxygen-ui';
import { ArrowRight } from '@wso2/oxygen-ui-icons-react';
import { useCallback, useState } from 'react';
import { defineMessages, useIntl } from 'react-intl';
import { useNavigate } from 'react-router-dom';
import { DefineApiPanel } from './components/DefineApiPanel';
import { GeneralCreateApiForm } from './components/GeneralCreateApiForm';
import { ApiCreationWizardDraftState, ApiType, GeneralApiCreationFormState } from './types';
import { ApiTypeSelector } from './components/ApiTypeSelector';
import { ApiCreationStepKey, ApiCreationSteps } from './components/ApiCreationSteps';
import { useCreateRestApi, useImportOpenApi } from '@/api/resources/restApis';
import { useConsoleScope } from '@/scope/ConsoleScopeProvider';
import { routes } from '@/routes/paths';
import { toCreateRestApiBody } from './utils/createRestApiBody';
import { toCreateApiFormErrors, type CreateApiFormErrors } from './utils/serverFieldErrors';
import {
  ApiCreationProgress,
  type ApiCreationProgressStatus,
} from './components/ApiCreationProgress';
import { API_TYPES } from './uiConfig';

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
  createAnApi: {
    id: 'api.create.ApiCreationWizard.title',
    defaultMessage: 'Create an API',
  },
  apiTypeSubtitle: {
    id: 'api.create.ApiCreationWizard.apiType.subtitle',
    defaultMessage: 'Choose how the gateway should expose your backend.',
  },
  apiTypeTitle: {
    id: 'api.create.ApiCreationWizard.apiType.title',
    defaultMessage: 'What kind of API are you exposing?',
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
    defaultMessage:
      'Bring an existing contract, or start from a blank slate and fill in the details yourself.',
  },
  sourceTitle: {
    id: 'api.create.ApiCreationWizard.source.title',
    defaultMessage: 'How do you want to define your {apiType}?',
    description:
      '{apiType} is the type picked in the first step, e.g. "REST API". Reads as one sentence.',
  },
  stepCount: {
    id: 'api.create.ApiCreationWizard.stepCount',
    defaultMessage: 'Step {current} of 3',
  },
});

export const ApiCreationWizard = () => {
  const intl = useIntl();
  const [step, setStep] = useState<ApiCreationStepKey>('apiType');
  const [apiType, setApiType] = useState<ApiType | null>(
    () => API_TYPES.find((candidate) => candidate.enabled) ?? null,
  );
  const [sourceDraft, setSourceDraft] = useState<ApiCreationWizardDraftState | null>(null);

  const [prefilledData, setPrefilledData] = useState<Partial<GeneralApiCreationFormState>>({});

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
    setPrefilledData(sourceDraft);
    setSubmittedValues(null);
    setStep('configure');
  };

  const navigate = useNavigate();
  // `handlesErrors`: a rejection this screen puts back on the form must not
  // also arrive as a snackbar that has faded by the time the user looks up.
  const createRestApiMutation = useCreateRestApi({ handlesErrors: true });
  const importOpenApiMutation = useImportOpenApi();
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
   * Why the last attempt was rejected, when the form is where it belongs.
   * Cleared on the next submission, not on the way back — the form is what
   * renders it, and it has to survive being returned to.
   */
  const [formErrors, setFormErrors] = useState<CreateApiFormErrors | null>(null);

  const createApi = (values: GeneralApiCreationFormState) => {
    const projectId = activeScope.projectHandler;
    if (!projectId) {
      // Nothing to create against — the wizard is mounted outside a project.
      return;
    }

    if (values.contractImport?.specFile) {
      // Import path: send multipart/form-data to POST /rest-apis/import-openapi.
      const formData = new FormData();
      formData.append('file', values.contractImport.specFile, 'openapi.json');
      formData.append('name', values.displayName.trim());
      formData.append('version', values.version.trim());
      // Normalize context to always have a leading slash, matching the standard create path.
      const context = `/${values.context.trim().replace(/^\/+/, '')}`;
      formData.append('context', context);
      formData.append('projectId', projectId);
      if (values.description?.trim()) {
        formData.append('description', values.description.trim());
      }
      const mainUrl = values.upstream?.main?.url?.trim();
      if (mainUrl) {
        formData.append('upstream', mainUrl);
      }
      importOpenApiMutation.mutate(formData);
    } else {
      setFormErrors(null);
      // Clears the previous attempt's error before the next one starts: the
      // progress screen reads its status from this mutation, and a stale
      // `isError` would show it as failed for the frame before the retry
      // registers as pending.
      createRestApiMutation.reset();
      createRestApiMutation.mutate(toCreateRestApiBody(values, { projectId }), {
        onError: (error) => {
          // A rejection the user can fix by editing goes straight back to the
          // form with the reason attached. Standing on a screen that says only
          // "we could not create this" would hide the one thing they need —
          // which value to change — behind a second click.
          const rejection = toCreateApiFormErrors(error);
          if (!rejection) return; // Not the form's to fix: the progress screen keeps it.

          setFormErrors(rejection);
          setCreationStarted(false);
        },
      });
    }
  };

  const onGeneralFormSumit = (finalData: GeneralApiCreationFormState) => {
    if (!activeScope.projectHandler) return;

    setSubmittedValues(finalData);
    setCreationStarted(true);
    createApi(finalData);
  };

  // Resolve the active mutation based on which path the current submission took.
  const isImportPath = Boolean(submittedValues?.contractImport);
  const activeMutation = isImportPath ? importOpenApiMutation : createRestApiMutation;

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
  }, [activeMutation.data?.id, navigate, params]);

  const creationStatus: ApiCreationProgressStatus = activeMutation.isError
    ? 'failed'
    : activeMutation.isSuccess
      ? 'created'
      : 'creating';

  if (creationStarted && submittedValues) {
    return (
      <ApiCreationProgress
        displayName={submittedValues.displayName}
        onBack={() => {
          createRestApiMutation.reset();
          importOpenApiMutation.reset();
          // Only the screen goes back; `submittedValues` stays so the form
          // returns to what was typed rather than to the imported draft.
          setCreationStarted(false);
        }}
        onComplete={goToCreatedApi}
        onRetry={() => createApi(submittedValues)}
        status={creationStatus}
      />
    );
  }

  const stepNumber = step === 'apiType' ? 1 : step === 'source' ? 2 : 3;

  return (
    <Box
      sx={{
        border: 1,
        borderColor: 'divider',
        borderRadius: 1,
        display: 'flex',
        flexDirection: 'column',
        minHeight: 620,
        overflow: 'hidden',
        width: '100%',
      }}
    >
      <Stack
        direction="row"
        spacing={2}
        sx={{ alignItems: 'center', borderBottom: 1, borderColor: 'divider', minHeight: 48, px: 3 }}
      >
        <Typography sx={{ fontWeight: 700, whiteSpace: 'nowrap' }} variant="body2">
          {intl.formatMessage(messages.createAnApi)}
        </Typography>
        <Divider flexItem orientation="vertical" sx={{ my: 1.5 }} />
        <ApiCreationSteps activeStep={step} onStepClick={(nextStep) => setStep(nextStep)} />
      </Stack>

      <Box sx={{ flex: 1, p: { md: 3.5, xs: 2 } }}>
        <Stack
          direction="column"
          spacing={step === 'configure' ? 2 : 3}
          sx={{ alignItems: 'flex-start' }}
        >
          <Box>
            <Typography variant="h1" sx={{ textAlign: 'left', mb: 1, fontWeight: 700 }}>
              {getTitleForStep(step)}
            </Typography>
            <Typography variant="body1" sx={{ textAlign: 'left' }}>
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

            {step !== 'apiType' && (
              <Box sx={{ display: step === 'source' ? 'block' : 'none' }}>
                {/* Kept mounted during configuration so Back preserves the selected source and edits. */}
                <DefineApiPanel initialApiTypeKey={apiType?.key} onDraftChange={setSourceDraft} />
              </Box>
            )}

            {step === 'configure' && (
              <Box sx={{ maxWidth: '80%' }}>
                <GeneralCreateApiForm
                  formId={CONFIGURE_FORM_ID}
                  hideActions
                  // What the user actually submitted, when there is such an
                  // attempt to come back from: the form remounts after the
                  // progress screen, so anything hand-typed would otherwise
                  // revert to the spec-derived draft.
                  initialValues={submittedValues ?? prefilledData}
                  onSubmit={onGeneralFormSumit}
                  onBack={() => setStep('source')}
                  serverErrors={formErrors ?? undefined}
                />
              </Box>
            )}
          </Box>
        </Stack>
      </Box>

      <Stack
        direction="row"
        sx={{
          alignItems: 'center',
          borderTop: 1,
          borderColor: 'divider',
          justifyContent: 'space-between',
          minHeight: 56,
          px: 3,
        }}
      >
        <Typography color="text.secondary" sx={{ fontWeight: 600 }} variant="caption">
          {intl.formatMessage(messages.stepCount, { current: stepNumber })}
        </Typography>
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
                if (step === 'apiType') setStep('source');
                else continueFromSource();
              }}
              type="button"
              variant="contained"
            >
              {intl.formatMessage(messages.continue)}
            </Button>
          )}
        </Stack>
      </Stack>
    </Box>
  );
};
